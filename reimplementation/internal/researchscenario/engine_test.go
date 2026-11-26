package researchscenario

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestPresets(t *testing.T) {
	cases := map[string][]Finding{
		"clean-series":     {},
		"duplicate-record": {{"DUPLICATE", 2}},
		"missing-record":   {{"MISSING", 2}},
		"out-of-order":     {{"OUT_OF_ORDER", 1}},
		"mixed-errors":     {{"DUPLICATE", 1}, {"OUT_OF_RANGE", 1}, {"MISSING", 2}},
	}
	if len(Catalog()) != len(cases) {
		t.Fatal("catalog size mismatch")
	}
	for _, definition := range Catalog() {
		t.Run(definition.ID, func(t *testing.T) {
			want, ok := cases[definition.ID]
			if !ok {
				t.Fatal("unexpected definition")
			}
			got, err := Run(definition.ID)
			if err != nil || !got.Passed || !reflect.DeepEqual(got.Findings, want) {
				t.Fatalf("report=%+v err=%v want=%+v", got, err, want)
			}
			if got.Version != definition.Version {
				t.Fatal("version mismatch")
			}
		})
	}
}

func TestRejectUnknownScenario(t *testing.T) {
	for _, id := range []string{"", "custom", "../clean-series", "CLEAN-SERIES"} {
		if _, err := Run(id); !errors.Is(err, ErrUnknownScenario) {
			t.Fatalf("accepted %q", id)
		}
	}
}

func TestDeterministicAndIndependent(t *testing.T) {
	a, _ := Run("mixed-errors")
	b, _ := Run("mixed-errors")
	if !reflect.DeepEqual(a, b) {
		t.Fatal("nondeterministic")
	}
	a.Records[0].Value = 99
	a.Findings[0].Code = "changed"
	a.Expected[0] = "changed"
	c, _ := Run("mixed-errors")
	if !reflect.DeepEqual(b, c) {
		t.Fatal("fixture mutated")
	}
}

func TestInvalidMeasurementAndSequence(t *testing.T) {
	got := inspect([]Observation{{0, 101}, {1, 0}, {2, 100}, {3, -1}})
	want := []Finding{{"INVALID_SEQUENCE", 0}, {"OUT_OF_RANGE", 0}, {"OUT_OF_RANGE", 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
}

func TestMissingAll(t *testing.T) {
	want := []Finding{{"MISSING", 1}, {"MISSING", 2}, {"MISSING", 3}}
	if got := inspect(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
}

func TestJSONUsesEmptyArrays(t *testing.T) {
	r, _ := Run("clean-series")
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"findings", "expected"} {
		if string(fields[key]) != "[]" {
			t.Fatalf("%s is %s", key, fields[key])
		}
	}
}
