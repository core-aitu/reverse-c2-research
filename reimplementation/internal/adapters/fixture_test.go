package adapters

import (
	"encoding/json"
	"testing"
)

func TestFixturesAreDeterministicAndSideEffectFree(t *testing.T) {
	input := json.RawMessage(`{"to":"research@example.test","subject":"preview"}`)
	first, err := ExecuteFixture("smtp", "preview", input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExecuteFixture("smtp", "preview", input)
	if err != nil {
		t.Fatal(err)
	}
	if first.Output["previewId"] != second.Output["previewId"] || first.Output["delivered"] != false {
		t.Fatalf("unexpected fixture output: %#v", first)
	}
}

func TestFixturesRejectUnknownOperations(t *testing.T) {
	if _, err := ExecuteFixture("ssh", "shell", json.RawMessage(`{"command":"id"}`)); err == nil {
		t.Fatal("shell fixture was accepted")
	}
}
