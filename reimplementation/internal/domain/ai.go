package domain

type AiAgentDefinition struct {
	ID                    int64    `json:"id"`
	AgentID               string   `json:"agentId"`
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	Prompt                string   `json:"prompt"`
	IncludeSpecterScripts bool     `json:"includeSpecterScripts"`
	AccessibleFunctions   []string `json:"accessibleFunctions"`
}

type AiFunctionDefinition struct {
	FunctionID    string `json:"functionId"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Documentation string `json:"documentation"`
	ApprovalLevel string `json:"approvalLevel"`
	Definition    string `json:"definition"`
}

type RunAiFunctionResult struct {
	Succeeded bool   `json:"succeeded"`
	Output    string `json:"output"`
}
