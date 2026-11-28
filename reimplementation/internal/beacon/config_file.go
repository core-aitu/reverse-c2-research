package beacon

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

const maxLaunchConfigSize = 64 << 10

// LaunchConfig is the secret-bearing, one-use configuration placed in a
// launch bundle. It is never copied into the persistent beacon State.
type LaunchConfig struct {
	Endpoint        string `json:"endpoint"`
	EnrollmentToken string `json:"enrollmentToken"`
	AgentID         string `json:"agentId,omitempty"`
	Name            string `json:"name,omitempty"`
	StatePath       string `json:"statePath,omitempty"`
	CABundle        string `json:"caBundle,omitempty"`
	Interval        string `json:"interval,omitempty"`
}

func LoadLaunchConfig(path string) (LaunchConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return LaunchConfig{}, err
	}
	defer clear(content)
	if len(content) == 0 || len(content) > maxLaunchConfigSize {
		return LaunchConfig{}, errors.New("launch config exceeds policy")
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var value LaunchConfig
	if err = decoder.Decode(&value); err != nil {
		return LaunchConfig{}, errors.New("launch config is invalid")
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return LaunchConfig{}, errors.New("launch config must contain one JSON value")
	}
	if value.Interval != "" {
		interval, parseErr := time.ParseDuration(value.Interval)
		if parseErr != nil || interval < 100*time.Millisecond || interval > 24*time.Hour {
			return LaunchConfig{}, errors.New("launch config interval is invalid")
		}
	}
	return value, nil
}
