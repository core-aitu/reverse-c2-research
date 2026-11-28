package capability

import (
	"fmt"
	"sort"
)

type State string

const (
	Registered      State = "registered"
	Fixture         State = "fixture"
	Implemented     State = "implemented"
	RuntimeVerified State = "runtime-verified"
	EvidenceBlocked State = "evidence-blocked"
)

type Definition struct {
	ID       string   `json:"id"`
	Group    string   `json:"group"`
	Title    string   `json:"title"`
	State    State    `json:"state"`
	Routes   []string `json:"routes"`
	Evidence string   `json:"evidence,omitempty"`
}

var definitions = []Definition{
	{ID: "management-plane", Group: "control", Title: "Authentication, settings, documentation and events", State: RuntimeVerified, Routes: []string{"/api/Accounts", "/api/Settings", "/api/Documentation", "/api/events"}},
	{ID: "typed-catalogs", Group: "control", Title: "Typed catalogs and network intelligence", State: RuntimeVerified, Routes: []string{"/api/Scripts", "/api/Wordlists", "/api/NetworkIntelligence"}},
	{ID: "research-modules", Group: "research", Title: "Scenario, journal, replay and report modules", State: RuntimeVerified, Routes: []string{"/api/research-scenarios"}},
	{ID: "artifact-content", Group: "content", Title: "Hashed artifact content storage", State: RuntimeVerified, Routes: []string{"GET /api/Artifacts/Artifacts/{host}/{name}", "PUT /api/artifacts/content/{host}/{name}", "/api/artifact-transfers"}},
	{ID: "update-packages", Group: "content", Title: "Verified update package storage and download", State: RuntimeVerified, Routes: []string{"GET /api/Updates/Updates/client", "/api/updates/packages"}},
	{ID: "update-installation", Group: "content", Title: "Signed update installation and rollback", State: Registered, Routes: []string{"POST /api/Updates"}},
	{ID: "listener-runtime", Group: "network", Title: "Listener socket lifecycle", State: Fixture, Routes: []string{"/api/Listeners"}},
	{ID: "tunnel-runtime", Group: "network", Title: "Tunnel forwarding lifecycle", State: Fixture, Routes: []string{"/api/Tunnels", "/api/SshTunnels", "/api/TorTunnels"}},
	{ID: "acme-runtime", Group: "service", Title: "ACME account and challenge lifecycle", State: Fixture, Routes: []string{"/api/AcmeAccount"}},
	{ID: "webdav-runtime", Group: "service", Title: "WebDAV content service", State: Fixture, Routes: []string{"/api/WebDavFiles"}},
	{ID: "tor-runtime", Group: "service", Title: "Tor process and circuit lifecycle", State: Fixture, Routes: []string{"/api/Tor"}},
	{ID: "ai-provider", Group: "service", Title: "External AI provider and MCP transport", State: Fixture, Routes: []string{"/api/AiSessions"}},
	{ID: "gateway-framing", Group: "gateway", Title: "Versioned, signed and bounded agent gateway", State: Implemented, Routes: []string{"/api/agent/v1", "/api/gateway/agents"}, Evidence: "research/validation/gateway-protocol/p2-validation.md"},
	{ID: "agent-safe-task-execution", Group: "gateway", Title: "Versioned command, output and remote-file task execution", State: Implemented, Routes: []string{"POST /api/gateway/agents/{id}/tasks", "POST /api/gateway/agents/{id}/tasks/{taskID}/cancel", "POST /api/agent/v1/task-output", "/api/agent/v1/files"}, Evidence: "research/validation/gateway-protocol/p2-validation.md"},
	{ID: "beacon-release-bundles", Group: "build", Title: "S3-backed beacon release matrix and one-use launch bundles", State: Implemented, Routes: []string{"GET /api/beacon-releases", "POST /api/beacon-builds", "POST /api/beacon-builds/{id}/bundle"}},
	{ID: "artifact-chunk-transfer", Group: "transfer", Title: "Digest-verified bounded artifact chunk transfer", State: Implemented, Routes: []string{"/api/artifact-transfers"}, Evidence: "research/validation/gateway-protocol/p1-validation.md"},
	{ID: "certificate-private-key-import", Group: "secret", Title: "Certificate private-key import", State: Registered, Routes: []string{"PUT /api/Certificates/Certificates/import"}},
	{ID: "agent-script-execution", Group: "gateway", Title: "Agent script execution", State: EvidenceBlocked, Routes: []string{"POST /api/tasks/runscript"}, Evidence: "research/validation/gaps.md#observed-session"},
	{ID: "loader-materialization", Group: "build", Title: "Loader bundle materialization", State: EvidenceBlocked, Routes: []string{"GET /api/Loaders/*"}, Evidence: "research/validation/gaps.md#payload-pipeline"},
	{ID: "pipeline-get-execution", Group: "build", Title: "Legacy state-changing pipeline execution", State: Registered, Routes: []string{"GET /api/PayloadPipeline/PayloadPipeline/{pipelineId}/run"}},
	{ID: "implant-download", Group: "build", Title: "Implant package download", State: EvidenceBlocked, Routes: []string{"POST /api/download"}, Evidence: "research/validation/gaps.md#payload-pipeline"},
	{ID: "phishing-delivery", Group: "delivery", Title: "SMTP phishing delivery", State: Registered, Routes: []string{"POST /api/Phishing/Send"}},
	{ID: "notebook-powershell-session", Group: "session", Title: "Notebook PowerShell interpreter session", State: Registered, Routes: []string{"/api/PowerShellNotebooks/{notebookId}/session"}},
	{ID: "powershell-command", Group: "session", Title: "PowerShell session command execution", State: Registered, Routes: []string{"POST /api/PowerShellSessions/{sessionId}/run"}},
	{ID: "ssh-command", Group: "session", Title: "SSH command execution", State: Registered, Routes: []string{"POST /api/SshSessions/{sessionId}/command"}},
	{ID: "ssh-upload", Group: "transfer", Title: "SSH file upload", State: Registered, Routes: []string{"POST /api/SshSessions/{sessionId}/upload"}},
	{ID: "ssh-download", Group: "transfer", Title: "SSH file download", State: Registered, Routes: []string{"GET /api/SshSessions/{sessionId}/download"}},
	{ID: "winrm-command", Group: "session", Title: "WinRM command execution", State: Registered, Routes: []string{"POST /api/WinRmSessions/{sessionId}/command"}},
	{ID: "winrm-upload", Group: "transfer", Title: "WinRM file upload", State: Registered, Routes: []string{"POST /api/WinRmSessions/{sessionId}/upload"}},
	{ID: "winrm-download", Group: "transfer", Title: "WinRM file download", State: Registered, Routes: []string{"GET /api/WinRmSessions/{sessionId}/download"}},
	{ID: "winrm-in-memory-load", Group: "build", Title: "WinRM in-memory loading", State: EvidenceBlocked, Routes: []string{"POST /api/WinRmSessions/{sessionId}/load/*"}, Evidence: "research/validation/gaps.md#payload-pipeline"},
	{ID: "cached-file-download", Group: "transfer", Title: "Remote cached-file retrieval", State: Registered, Routes: []string{"GET /api/NetworkIntelligence/DownloadCachedFileContent"}},
}

var byID map[string]Definition

func init() {
	if err := Validate(definitions); err != nil {
		panic(err)
	}
	byID = make(map[string]Definition, len(definitions))
	for _, definition := range definitions {
		byID[definition.ID] = clone(definition)
	}
}

func All() []Definition {
	result := make([]Definition, len(definitions))
	for index, definition := range definitions {
		result[index] = clone(definition)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result
}

func Get(id string) (Definition, bool) {
	definition, ok := byID[id]
	return clone(definition), ok
}

func Validate(values []Definition) error {
	states := map[State]bool{Registered: true, Fixture: true, Implemented: true, RuntimeVerified: true, EvidenceBlocked: true}
	seen := map[string]bool{}
	for _, value := range values {
		if value.ID == "" || value.Group == "" || value.Title == "" || len(value.Routes) == 0 {
			return fmt.Errorf("capability has incomplete identity: %q", value.ID)
		}
		if seen[value.ID] {
			return fmt.Errorf("duplicate capability id: %s", value.ID)
		}
		if !states[value.State] {
			return fmt.Errorf("capability %s has invalid state %q", value.ID, value.State)
		}
		if value.State == EvidenceBlocked && value.Evidence == "" {
			return fmt.Errorf("evidence-blocked capability %s has no evidence reference", value.ID)
		}
		seen[value.ID] = true
	}
	return nil
}

func clone(value Definition) Definition {
	value.Routes = append([]string(nil), value.Routes...)
	return value
}
