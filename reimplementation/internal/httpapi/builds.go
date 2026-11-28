package httpapi

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/research/specter-reimplementation/internal/store"
)

const maxBeaconRelease = 128 << 20

func (s *Server) beaconBuildRoutes() {
	s.mux.Handle("GET /api/beacon-releases", s.protected(http.HandlerFunc(s.beaconReleases)))
	s.mux.Handle("PUT /api/beacon-releases/{version}/{os}/{arch}", s.requireRole("admin", http.HandlerFunc(s.putBeaconRelease)))
	s.mux.Handle("PUT /api/beacon-releases/{id}/assets/{asset}", s.requireRole("admin", http.HandlerFunc(s.putBeaconReleaseAsset)))
	s.mux.Handle("POST /api/beacon-builds", s.requireRole("admin", http.HandlerFunc(s.createBeaconBuild)))
	s.mux.Handle("POST /api/beacon-builds/{id}/bundle", s.requireRole("admin", http.HandlerFunc(s.downloadBeaconBundle)))
}

func (s *Server) beaconReleases(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.BeaconReleases(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) putBeaconRelease(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength <= 0 || r.ContentLength > maxBeaconRelease {
		writeError(w, http.StatusBadRequest, "beacon release size exceeds policy")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBeaconRelease)
	operatingSystem, architecture := r.PathValue("os"), r.PathValue("arch")
	name := "specter-beacon-" + operatingSystem + "-" + architecture
	if operatingSystem == "windows" {
		name += ".exe"
	}
	object, err := s.store.PutObject(r.Context(), "beacon-release", name, "application/octet-stream", r.Body, r.ContentLength)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	expected := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Content-SHA256")))
	if expected != "" && (len(expected) != 64 || subtle.ConstantTimeCompare([]byte(expected), []byte(object.SHA256)) != 1) {
		writeError(w, http.StatusBadRequest, "X-Content-SHA256 does not match beacon release")
		return
	}
	development, _ := strconv.ParseBool(r.Header.Get("X-Beacon-Development"))
	value, err := s.store.RegisterBeaconRelease(r.Context(), newID("release"), r.PathValue("version"), operatingSystem, architecture, object.ID, development)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "BeaconReleasePublished", value.ID, value.Version+"; "+value.OS+"/"+value.Architecture+"; "+value.Object.SHA256)
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) putBeaconReleaseAsset(w http.ResponseWriter, r *http.Request) {
	release, err := s.store.BeaconRelease(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "beacon release not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	asset := r.PathValue("asset")
	limit, kind, name, mediaType := int64(0), "", "", requestMediaType(r)
	switch asset {
	case "sbom":
		limit, kind, name = 16<<20, "beacon-sbom", "specter-beacon.spdx.json"
		if mediaType == "application/octet-stream" {
			mediaType = "application/spdx+json"
		}
	case "checksums":
		limit, kind, name = 4<<20, "beacon-checksums", "SHA256SUMS"
	case "signature":
		limit, kind, name = 4<<20, "beacon-signature", "SHA256SUMS.sig"
	default:
		writeError(w, http.StatusBadRequest, "unsupported beacon release asset")
		return
	}
	if r.ContentLength <= 0 || r.ContentLength > limit {
		writeError(w, http.StatusBadRequest, "beacon release asset size exceeds policy")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	content, err := io.ReadAll(r.Body)
	if err != nil || int64(len(content)) != r.ContentLength {
		writeError(w, http.StatusBadRequest, "beacon release asset is incomplete")
		return
	}
	if asset == "checksums" && !checksumManifestContains(content, release.Object.SHA256, release.Object.Name) {
		writeError(w, http.StatusConflict, "checksum manifest does not reference the release binary")
		return
	}
	if asset == "sbom" {
		var document struct {
			SPDXVersion string `json:"spdxVersion"`
			Name        string `json:"name"`
		}
		if json.Unmarshal(content, &document) != nil || document.SPDXVersion != "SPDX-2.3" || document.Name == "" {
			writeError(w, http.StatusBadRequest, "SBOM must be a valid SPDX 2.3 JSON document")
			return
		}
	}
	object, err := s.store.PutObject(r.Context(), kind, name, mediaType, bytes.NewReader(content), int64(len(content)))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	value, err := s.store.SetBeaconReleaseAsset(r.Context(), release.ID, asset, object.ID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), principal(r).Username, "BeaconReleaseAssetPublished", release.ID, asset+"; "+object.SHA256)
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createBeaconBuild(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ReleaseID string `json:"releaseId"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	release, err := s.store.BeaconRelease(r.Context(), request.ReleaseID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "beacon release not found")
		return
	}
	if s.cfg.RequireSignedReleases && !release.Signed {
		writeError(w, http.StatusConflict, "release signature is required")
		return
	}
	value, err := s.store.CreateBeaconBuild(r.Context(), newID("build"), request.ReleaseID, principal(r).Username)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), principal(r).Username, "BeaconBuildCreated", value.ID, value.Version+"; "+value.OS+"/"+value.Architecture)
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) downloadBeaconBundle(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AgentPublicURL == "" {
		writeError(w, http.StatusServiceUnavailable, "SPECTER_AGENT_PUBLIC_URL is not configured")
		return
	}
	build, err := s.store.BeaconBuild(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "beacon build not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	release, err := s.store.BeaconRelease(r.Context(), build.ReleaseID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if s.cfg.RequireSignedReleases && !release.Signed {
		writeError(w, http.StatusConflict, "release signature is required")
		return
	}
	binary, _, err := s.store.OpenObject(r.Context(), release.Object.ID, 0, -1)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "beacon release object unavailable")
		return
	}
	defer binary.Close()
	token, err := secureToken()
	if err != nil {
		s.internalError(w, err)
		return
	}
	agentID, enrollmentID := newID("agent"), newID("enroll")
	expires := time.Now().UTC().Add(10 * time.Minute)
	if err = s.store.CreateAgentEnrollment(r.Context(), enrollmentID, token, principal(r).Username, expires); err != nil {
		s.internalError(w, err)
		return
	}
	caName, caContent := "", []byte(nil)
	if s.cfg.AgentCABundle != "" {
		caContent, err = os.ReadFile(filepath.Clean(s.cfg.AgentCABundle))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "agent CA bundle unavailable")
			return
		}
		caName = "agent-ca.pem"
	}
	config := map[string]any{
		"endpoint": s.cfg.AgentPublicURL, "enrollmentToken": token, "agentId": agentID,
		"name": agentID, "statePath": "beacon-state.json", "interval": "5s", "caBundle": caName,
	}
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	runSH := []byte("#!/bin/sh\nexec ./specter-beacon -config ./specter-beacon.json\n")
	runPS := []byte("& .\\specter-beacon.exe -config .\\specter-beacon.json\n")
	files := map[string]string{fileNameForRelease(release.OS): release.Object.SHA256, "specter-beacon.json": sha256Text(configJSON), "run.sh": sha256Text(runSH), "run.ps1": sha256Text(runPS)}
	if len(caContent) > 0 {
		files[caName] = sha256Text(caContent)
	}
	manifest := map[string]any{
		"version": release.Version, "os": release.OS, "architecture": release.Architecture,
		"sha256": release.Object.SHA256, "binarySize": release.Object.Size, "enrollmentExpiresAt": expires, "files": files,
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	fileName := fileNameForRelease(release.OS)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="specter-beacon-%s-%s-%s.zip"`, release.Version, release.OS, release.Architecture))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	archive := zip.NewWriter(w)
	writeZipStream := func(name string, mode os.FileMode, source io.Reader) error {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(mode)
		entry, createErr := archive.CreateHeader(header)
		if createErr != nil {
			return createErr
		}
		_, copyErr := io.Copy(entry, source)
		return copyErr
	}
	if err = writeZipStream(fileName, 0755, binary); err == nil {
		err = writeZipStream("specter-beacon.json", 0600, strings.NewReader(string(configJSON)))
	}
	if err == nil && len(caContent) > 0 {
		err = writeZipStream(caName, 0644, strings.NewReader(string(caContent)))
	}
	if err == nil {
		err = writeZipStream("manifest.json", 0644, strings.NewReader(string(manifestJSON)))
	}
	if err == nil {
		err = writeZipStream("run.sh", 0755, strings.NewReader(string(runSH)))
	}
	if err == nil {
		err = writeZipStream("run.ps1", 0644, strings.NewReader(string(runPS)))
	}
	closeErr := archive.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		s.log.Error("beacon bundle stream failed", "build", build.ID, "error", err)
		return
	}
	now := time.Now().UTC()
	_ = s.store.MarkBeaconBundle(r.Context(), build.ID, now)
	s.audit(r.Context(), principal(r).Username, "BeaconBundleDownloaded", build.ID, release.Version+"; "+release.OS+"/"+release.Architecture)
}

func fileNameForRelease(operatingSystem string) string {
	if operatingSystem == "windows" {
		return "specter-beacon.exe"
	}
	return "specter-beacon"
}

func sha256Text(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func checksumManifestContains(content []byte, expectedDigest, expectedName string) bool {
	expectedDigest = strings.ToLower(expectedDigest)
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == expectedName && strings.ToLower(fields[0]) == expectedDigest {
			return true
		}
	}
	return false
}
