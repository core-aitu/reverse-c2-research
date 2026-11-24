package pipeline

import (
	"encoding/json"
	"testing"
)

func TestSafeFixtureDAG(t *testing.T) {
	definition, err := Parse(json.RawMessage(`{"version":1,"nodes":[{"id":"source","type":"input","dependsOn":[]},{"id":"hash","type":"sha256","dependsOn":["source"]},{"id":"encoded","type":"base64","dependsOn":["source"]}],"outputs":["hash","encoded"]}`))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(definition, "specter")
	if err != nil {
		t.Fatal(err)
	}
	if result.Outputs["encoded"] != "c3BlY3Rlcg==" || len(result.Outputs["hash"]) != 64 {
		t.Fatalf("unexpected outputs: %#v", result.Outputs)
	}
}

func TestRejectsCyclesAndExecutionNodes(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"nodes":[{"id":"a","type":"concat","dependsOn":["b"]},{"id":"b","type":"concat","dependsOn":["a"]}],"outputs":["a"]}`,
		`{"version":1,"nodes":[{"id":"a","type":"shell","dependsOn":[]}],"outputs":["a"]}`,
	} {
		definition, err := Parse(json.RawMessage(raw))
		if err == nil {
			_, err = Validate(definition)
		}
		if err == nil {
			t.Fatalf("expected rejection for %s", raw)
		}
	}
}
