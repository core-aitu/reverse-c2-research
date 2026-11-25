package worker

import (
	"testing"

	"github.com/research/specter-reimplementation/internal/pipeline"
)

func TestRunSafePipeline(t *testing.T) {
	response := Run(Request{Version: 1, Input: "specter", Definition: pipeline.Definition{Version: 1,
		Nodes:   []pipeline.Node{{ID: "input", Type: "input"}, {ID: "hash", Type: "sha256", DependsOn: []string{"input"}}},
		Outputs: []string{"hash"}}})
	if response.Error != "" || response.Result.Outputs["hash"] != "1b2ee055072ad7f68c05a828393967d407ea160383daae962112166979008bac" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestRunRejectsExecutableNode(t *testing.T) {
	response := Run(Request{Version: 1, Definition: pipeline.Definition{Version: 1,
		Nodes: []pipeline.Node{{ID: "shell", Type: "shell", Value: "id"}}, Outputs: []string{"shell"}}})
	if response.Error == "" {
		t.Fatal("executable node was accepted")
	}
}
