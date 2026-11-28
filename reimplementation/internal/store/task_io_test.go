package store

import (
	"testing"

	"github.com/research/specter-reimplementation/internal/domain"
)

func TestJoinTaskOutputRequiresContiguousEOF(t *testing.T) {
	valid := []domain.TaskOutputChunk{
		{Stream: "stdout", Index: 0, Content: []byte("one-")},
		{Stream: "stdout", Index: 1, Content: []byte("two"), EOF: true, Truncated: true},
	}
	content, truncated, err := joinTaskOutput(valid, "stdout")
	if err != nil || string(content) != "one-two" || !truncated {
		t.Fatalf("content=%q truncated=%v err=%v", content, truncated, err)
	}
	for name, values := range map[string][]domain.TaskOutputChunk{
		"gap":        {{Stream: "stdout", Index: 1, EOF: true}},
		"missingEOF": {{Stream: "stdout", Index: 0, Content: []byte("partial")}},
		"afterEOF": {
			{Stream: "stdout", Index: 0, EOF: true},
			{Stream: "stdout", Index: 1, EOF: true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := joinTaskOutput(values, "stdout"); err == nil {
				t.Fatal("invalid output sequence accepted")
			}
		})
	}
}
