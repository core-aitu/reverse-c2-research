//go:build e2e_fixture

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const managementURL = "http://server:8080"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	password := os.Getenv("SPECTER_ADMIN_PASSWORD")
	if password == "" {
		panic("SPECTER_ADMIN_PASSWORD is required")
	}
	waitReady(ctx)
	token := authenticate(ctx, password)
	binary := "/out/specter-beacon"
	release := uploadRelease(ctx, token, binary)
	bundle := downloadBundle(ctx, token, release.ID)
	directory, err := os.MkdirTemp("", "specter-e2e-bundle-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(directory)
	if err = extractBundle(bundle, directory); err != nil {
		panic(err)
	}
	launch := validateBundle(directory)
	remotePath := "/tmp/specter-e2e-remote.txt"
	if err = os.WriteFile(remotePath, []byte("playwright-remote-file\n"), 0600); err != nil {
		panic(err)
	}
	defer os.Remove(remotePath)
	beaconPath := filepath.Join(directory, "specter-beacon")
	command := exec.CommandContext(ctx, beaconPath,
		"-config", filepath.Join(directory, "specter-beacon.json"),
		"-endpoint", "https://server:8443",
		"-ca-bundle", filepath.Join(directory, "agent-ca.pem"),
		"-interval", "500ms", "-jitter", "10ms", "-state", filepath.Join(directory, "beacon-state.json"),
	)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err = command.Start(); err != nil {
		panic(err)
	}
	waitEnrolled(ctx, filepath.Join(directory, "beacon-state.json"))
	assertEnrollmentTokenIsSingleUse(directory, launch.EnrollmentToken)
	<-ctx.Done()
	if command.Process != nil {
		_ = command.Process.Signal(os.Interrupt)
	}
	if err = command.Wait(); err != nil && !errors.Is(ctx.Err(), context.Canceled) {
		panic(err)
	}
}

type releaseResponse struct {
	ID string `json:"id"`
}

type launchConfig struct {
	EnrollmentToken string `json:"enrollmentToken"`
}

func validateBundle(directory string) launchConfig {
	manifestContent, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		panic(err)
	}
	var manifest struct {
		Files map[string]string `json:"files"`
	}
	if json.Unmarshal(manifestContent, &manifest) != nil || len(manifest.Files) != 5 {
		panic("bundle manifest is invalid")
	}
	for _, name := range []string{"specter-beacon", "specter-beacon.json", "agent-ca.pem", "run.sh", "run.ps1"} {
		expected := manifest.Files[name]
		content, readErr := os.ReadFile(filepath.Join(directory, name))
		digest := sha256.Sum256(content)
		if readErr != nil || expected == "" || hex.EncodeToString(digest[:]) != expected {
			panic("bundle manifest digest mismatch for " + name)
		}
	}
	configContent, err := os.ReadFile(filepath.Join(directory, "specter-beacon.json"))
	if err != nil {
		panic(err)
	}
	var launch launchConfig
	if json.Unmarshal(configContent, &launch) != nil || launch.EnrollmentToken == "" {
		panic("bundle launch configuration omitted enrollment token")
	}
	if bytes.Contains(manifestContent, []byte(launch.EnrollmentToken)) {
		panic("bundle manifest exposed enrollment token")
	}
	return launch
}

func waitEnrolled(ctx context.Context, statePath string) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(statePath)
		var state struct {
			Identity string `json:"identity"`
		}
		if err == nil && json.Unmarshal(content, &state) == nil && state.Identity != "" {
			return
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	panic("beacon did not persist enrolled identity")
}

func assertEnrollmentTokenIsSingleUse(directory, token string) {
	reuseContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(reuseContext, filepath.Join(directory, "specter-beacon"),
		"-config", filepath.Join(directory, "specter-beacon.json"),
		"-endpoint", "https://server:8443",
		"-ca-bundle", filepath.Join(directory, "agent-ca.pem"),
		"-agent-id", "reused-token-agent",
		"-state", filepath.Join(directory, "reused-token-state.json"),
		"-interval", "500ms", "-jitter", "10ms",
	)
	output, err := command.CombinedOutput()
	if err == nil || reuseContext.Err() != nil {
		panic(fmt.Sprintf("enrollment token reuse was not rejected: token_length=%d output=%s", len(token), output))
	}
}

func waitReady(ctx context.Context) {
	for {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, managementURL+"/readyz", nil)
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func authenticate(ctx context.Context, password string) string {
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": password})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, managementURL+"/api/Accounts/authenticate", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	var result struct {
		JWTToken string `json:"jwtToken"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&result) != nil || result.JWTToken == "" {
		panic("fixture authentication failed")
	}
	return result.JWTToken
}

func uploadRelease(ctx context.Context, token, path string) releaseResponse {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodPut, managementURL+"/api/beacon-releases/e2e/linux/amd64", bytes.NewReader(content))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/octet-stream")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	var release releaseResponse
	if response.StatusCode != http.StatusCreated || json.NewDecoder(response.Body).Decode(&release) != nil || release.ID == "" {
		content, _ := io.ReadAll(response.Body)
		panic(fmt.Sprintf("release publication failed: %d %s", response.StatusCode, content))
	}
	return release
}

func downloadBundle(ctx context.Context, token, releaseID string) []byte {
	body, _ := json.Marshal(map[string]string{"releaseId": releaseID})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, managementURL+"/api/beacon-builds", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		panic(err)
	}
	var build struct {
		ID string `json:"id"`
	}
	if response.StatusCode != http.StatusCreated || json.NewDecoder(response.Body).Decode(&build) != nil || build.ID == "" {
		response.Body.Close()
		panic("build creation failed")
	}
	response.Body.Close()
	request, _ = http.NewRequestWithContext(ctx, http.MethodPost, managementURL+"/api/beacon-builds/"+build.ID+"/bundle", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 256<<20))
	if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(response.Header.Get("Content-Type"), "application/zip") {
		panic("bundle download failed")
	}
	return content
}

func extractBundle(content []byte, directory string) error {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return err
	}
	for _, file := range archive.File {
		if file.Name != filepath.Base(file.Name) {
			return errors.New("unsafe bundle entry")
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		path := filepath.Join(directory, file.Name)
		output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, file.Mode())
		if err != nil {
			reader.Close()
			return err
		}
		_, err = io.Copy(output, reader)
		closeErr := output.Close()
		reader.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
