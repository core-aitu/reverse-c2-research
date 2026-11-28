package capability

import "testing"

func TestCatalogIsValidAndDefensivelyCopied(t *testing.T) {
	values := All()
	if len(values) < 20 {
		t.Fatalf("expected a complete capability catalog, got %d entries", len(values))
	}
	if err := Validate(values); err != nil {
		t.Fatal(err)
	}
	values[0].Routes[0] = "mutated"
	again := All()
	if again[0].Routes[0] == "mutated" {
		t.Fatal("catalog routes were not defensively copied")
	}
}

func TestEvidenceBlockedEntriesHaveReferences(t *testing.T) {
	for _, value := range All() {
		if value.State == EvidenceBlocked && value.Evidence == "" {
			t.Errorf("%s has no evidence reference", value.ID)
		}
	}
}

func TestGatewayCapabilitiesAwaitFullSoakBeforeRuntimeVerification(t *testing.T) {
	for _, id := range []string{"gateway-framing", "agent-safe-task-execution"} {
		value, ok := Get(id)
		if !ok {
			t.Fatalf("missing gateway capability %s", id)
		}
		if value.State != Implemented {
			t.Fatalf("%s state=%s; runtime verification requires the recorded multi-hour gate", id, value.State)
		}
	}
}
