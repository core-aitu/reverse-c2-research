package beacon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLaunchConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "specter-beacon.json")
	content := []byte(`{"endpoint":"https://gateway.example.test:8443","enrollmentToken":"one-use","agentId":"agent-1","name":"lab","statePath":"state.json","caBundle":"ca.pem","interval":"5s"}`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	value, err := LoadLaunchConfig(path)
	if err != nil || value.Endpoint != "https://gateway.example.test:8443" || value.EnrollmentToken != "one-use" || value.Interval != "5s" {
		t.Fatalf("value=%#v err=%v", value, err)
	}
}

func TestLoadLaunchConfigRejectsUnknownAndBadDuration(t *testing.T) {
	for _, content := range []string{`{"endpoint":"https://example.test","unknown":true}`, `{"endpoint":"https://example.test","interval":"fast"}`, `{}`} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		value, err := LoadLaunchConfig(path)
		if content == `{}` {
			if err != nil || value.Endpoint != "" {
				t.Fatalf("empty optional config: value=%#v err=%v", value, err)
			}
		} else if err == nil {
			t.Fatalf("accepted %s", content)
		}
	}
}
