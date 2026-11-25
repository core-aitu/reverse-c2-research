package researchjournal

import (
	"encoding/json"
	"github.com/research/specter-reimplementation/internal/researchscenario"
	"strings"
	"testing"
)

func TestAllPresets(t *testing.T) {
	for _, preset := range researchscenario.Catalog() {
		t.Run(preset.ID, func(t *testing.T) {
			report, err := researchscenario.Run(preset.ID)
			if err != nil {
				t.Fatal(err)
			}
			entries := FromReport(report)
			if len(entries) != len(report.Findings)+2 {
				t.Fatal("wrong event count")
			}
			if entries[0].Kind != "dataset.checked" {
				t.Fatal("missing first entry")
			}
			last := entries[len(entries)-1]
			if last.Kind != "report.completed" || last.Passed == nil || *last.Passed != report.Passed {
				t.Fatal("wrong final entry")
			}
			for i, entry := range entries {
				if entry.Ordinal != i+1 {
					t.Fatal("incorrect ordinal")
				}
			}
			for i, finding := range report.Findings {
				entry := entries[i+1]
				if entry.Kind != "finding.recorded" || entry.Code == nil || *entry.Code != finding.Code || entry.Sequence == nil || *entry.Sequence != finding.Sequence {
					t.Fatal("incorrect finding")
				}
			}
		})
	}
}

func TestFalseAndZeroAreNotOmitted(t *testing.T) {
	report := researchscenario.Report{Findings: []researchscenario.Finding{{Code: "INVALID_SEQUENCE", Sequence: 0}}, Passed: false}
	encoded, err := json.Marshal(FromReport(report))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"passed":false`) || !strings.Contains(string(encoded), `"sequence":0`) {
		t.Fatal(string(encoded))
	}
}

func TestRepeatedFindingsArePreserved(t *testing.T) {
	r := researchscenario.Report{Findings: []researchscenario.Finding{{Code: "DUPLICATE", Sequence: 1}, {Code: "DUPLICATE", Sequence: 1}}}
	a := FromReport(r)
	if len(a) != 4 || a[1].Ordinal == a[2].Ordinal {
		t.Fatal("lost repeated finding")
	}
	*a[1].Code = "changed"
	if *a[2].Code != "DUPLICATE" || r.Findings[0].Code != "DUPLICATE" {
		t.Fatal("aliased entries")
	}
}
