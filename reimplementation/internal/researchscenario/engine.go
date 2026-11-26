// Package researchscenario validates fixed, fictional measurement datasets.
// It has no access to the gateway, worker, network or filesystem.
package researchscenario

import (
	"errors"
	"sort"
)

const Version = 1

var ErrUnknownScenario = errors.New("unknown research scenario")

type Definition struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     int    `json:"version"`
}

type Observation struct {
	Sequence int `json:"sequence"`
	Value    int `json:"value"`
}

type Finding struct {
	Code     string `json:"code"`
	Sequence int    `json:"sequence"`
}

type Report struct {
	ScenarioID string        `json:"scenarioId"`
	Version    int           `json:"version"`
	Records    []Observation `json:"records"`
	Findings   []Finding     `json:"findings"`
	Expected   []string      `json:"expected"`
	Passed     bool          `json:"passed"`
}

type preset struct {
	definition Definition
	records    []Observation
	expected   []string
}

// Construct fresh slices on every call: runs cannot modify later fixtures.
func presets() []preset {
	return []preset{
		{Definition{"clean-series", "Complete series", "Three consecutive fictional measurements.", Version}, []Observation{{1, 12}, {2, 15}, {3, 14}}, []string{}},
		{Definition{"duplicate-record", "Duplicate observation", "A repeated sequence number should be flagged.", Version}, []Observation{{1, 12}, {2, 15}, {2, 15}, {3, 14}}, []string{"DUPLICATE"}},
		{Definition{"missing-record", "Missing observation", "A gap in the fixed three-record series should be flagged.", Version}, []Observation{{1, 12}, {3, 14}}, []string{"MISSING"}},
		{Definition{"out-of-order", "Out-of-order observations", "A descending sequence should be flagged without changing the source order.", Version}, []Observation{{2, 15}, {1, 12}, {3, 14}}, []string{"OUT_OF_ORDER"}},
		{Definition{"mixed-errors", "Multiple quality issues", "A duplicate, a missing observation and an invalid measurement.", Version}, []Observation{{1, 12}, {1, -1}, {3, 14}}, []string{"DUPLICATE", "MISSING", "OUT_OF_RANGE"}},
	}
}

func Catalog() []Definition {
	items := presets()
	out := make([]Definition, 0, len(items))
	for _, item := range items {
		out = append(out, item.definition)
	}
	return out
}

// Run is deterministic. The caller can select only a built-in fixture.
func Run(id string) (Report, error) {
	for _, item := range presets() {
		if item.definition.ID != id {
			continue
		}
		findings := inspect(item.records)
		actual := make([]string, 0, len(findings))
		for _, finding := range findings {
			actual = append(actual, finding.Code)
		}
		sort.Strings(actual)
		return Report{ScenarioID: id, Version: Version, Records: item.records,
			Findings: findings, Expected: item.expected, Passed: equal(actual, item.expected)}, nil
	}
	return Report{}, ErrUnknownScenario
}

// The fictional instrument is specified to emit exactly sequences 1..3,
// with measurement values between 0 and 100 inclusive.
func inspect(records []Observation) []Finding {
	out := make([]Finding, 0)
	seen := map[int]bool{}
	previous := 0
	for _, record := range records {
		seq := record.Sequence
		if seq < 1 || seq > 3 {
			out = append(out, Finding{"INVALID_SEQUENCE", seq})
		} else if seen[seq] {
			out = append(out, Finding{"DUPLICATE", seq})
		}
		if seq < previous {
			out = append(out, Finding{"OUT_OF_ORDER", seq})
		}
		if record.Value < 0 || record.Value > 100 {
			out = append(out, Finding{"OUT_OF_RANGE", seq})
		}
		seen[seq] = true
		previous = seq
	}
	for seq := 1; seq <= 3; seq++ {
		if !seen[seq] {
			out = append(out, Finding{"MISSING", seq})
		}
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
