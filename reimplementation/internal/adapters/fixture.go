package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

type FixtureResult struct {
	Adapter   string         `json:"adapter"`
	Operation string         `json:"operation"`
	Output    map[string]any `json:"output"`
}

func ExecuteFixture(name, operation string, input json.RawMessage) (FixtureResult, error) {
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	if len(input) > 64<<10 || !json.Valid(input) {
		return FixtureResult{}, errors.New("fixture input must be valid JSON up to 64 KiB")
	}
	var values map[string]any
	if err := json.Unmarshal(input, &values); err != nil || values == nil {
		return FixtureResult{}, errors.New("fixture input must be a JSON object")
	}
	result := FixtureResult{Adapter: name, Operation: operation}
	switch name + "/" + operation {
	case "elasticsearch/status":
		result.Output = map[string]any{"available": true, "documents": 0, "mode": "fixture"}
	case "smtp/preview":
		to, _ := values["to"].(string)
		subject, _ := values["subject"].(string)
		if !strings.Contains(to, "@") || len(to) > 320 || len(subject) > 998 {
			return FixtureResult{}, errors.New("invalid SMTP preview")
		}
		result.Output = map[string]any{"accepted": false, "delivered": false, "previewId": stableID(name, input)}
	case "acme/order-preview":
		commonName, _ := values["commonName"].(string)
		if commonName == "" || len(commonName) > 253 || strings.ContainsAny(commonName, "/\\") {
			return FixtureResult{}, errors.New("invalid ACME preview common name")
		}
		result.Output = map[string]any{"created": false, "commonName": commonName, "orderId": stableID(name, input)}
	case "webdav/list":
		result.Output = map[string]any{"path": "/", "entries": []any{}}
	case "ai-provider/models":
		result.Output = map[string]any{"models": []string{"fixture-deterministic"}}
	case "ssh/inventory", "winrm/inventory":
		result.Output = map[string]any{"connected": false, "hosts": []any{}, "mode": "fixture"}
	case "tor/status":
		result.Output = map[string]any{"connected": false, "circuits": 0, "mode": "fixture"}
	case "update-service/check":
		result.Output = map[string]any{"available": false, "channel": "fixture", "version": "7.0.0-clean-room"}
	default:
		return FixtureResult{}, errors.New("unsupported fixture adapter operation")
	}
	return result, nil
}

func stableID(name string, input json.RawMessage) string {
	digest := sha256.Sum256(append([]byte(name+":"), input...))
	return "fixture-" + hex.EncodeToString(digest[:8])
}
