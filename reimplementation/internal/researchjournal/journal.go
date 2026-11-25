// Package researchjournal projects fictional measurement reports into a read-only timeline.
// It does not accept external events, execute tasks, or publish to the operational event hub.
package researchjournal

import "github.com/research/specter-reimplementation/internal/researchscenario"

type Entry struct {
	Ordinal  int     `json:"ordinal"`
	Kind     string  `json:"kind"`
	Code     *string `json:"code,omitempty"`
	Sequence *int    `json:"sequence,omitempty"`
	Passed   *bool   `json:"passed,omitempty"`
}

// FromReport preserves the order of findings, including repeated findings.
func FromReport(report researchscenario.Report) []Entry {
	entries := make([]Entry, 0, len(report.Findings)+2)
	entries = append(entries, Entry{Ordinal: 1, Kind: "dataset.checked"})
	for _, finding := range report.Findings {
		code, sequence := finding.Code, finding.Sequence
		entries = append(entries, Entry{Ordinal: len(entries) + 1, Kind: "finding.recorded", Code: &code, Sequence: &sequence})
	}
	passed := report.Passed
	return append(entries, Entry{Ordinal: len(entries) + 1, Kind: "report.completed", Passed: &passed})
}
