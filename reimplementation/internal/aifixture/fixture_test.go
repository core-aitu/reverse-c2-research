package aifixture

import "testing"

func TestFixtureOperations(t *testing.T) {
	echo, err := Parse(`{"version":1,"operation":"echo","argument":"text"}`)
	if err != nil {
		t.Fatal(err)
	}
	output, err := Run(echo, map[string]string{"text": "fixture-live-window"})
	if err != nil || output != "fixture-live-window" {
		t.Fatalf("echo output=%q err=%v", output, err)
	}
	hash, err := Parse(`{"version":1,"operation":"sha256","argument":"text"}`)
	if err != nil {
		t.Fatal(err)
	}
	output, err = Run(hash, map[string]string{"text": "specter"})
	if err != nil || output != "1b2ee055072ad7f68c05a828393967d407ea160383daae962112166979008bac" {
		t.Fatalf("hash output=%q err=%v", output, err)
	}
}

func TestFixtureRejectsExecutionShapes(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"operation":"shell","argument":"command"}`,
		`{"version":1,"operation":"echo","argument":"../secret"}`,
		`{"version":1,"operation":"echo","argument":"text","network":true}`,
	} {
		if _, err := Parse(raw); err == nil {
			t.Fatalf("accepted unsafe definition %s", raw)
		}
	}
}
