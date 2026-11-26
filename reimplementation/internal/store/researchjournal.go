package store

import (
	"context"
	"github.com/research/specter-reimplementation/internal/researchjournal"
)

// ResearchJournal returns only entries derived from a stored fictional report.
func (s *Store) ResearchJournal(ctx context.Context, runID string) ([]researchjournal.Entry, error) {
	rows, err := s.pool.Query(ctx, `SELECT ordinal, kind, code, sequence, passed
        FROM app_research_journal WHERE run_id=$1 ORDER BY ordinal`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]researchjournal.Entry, 0)
	for rows.Next() {
		var entry researchjournal.Entry
		if err := rows.Scan(&entry.Ordinal, &entry.Kind, &entry.Code, &entry.Sequence, &entry.Passed); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, ErrNotFound
	}
	return entries, nil
}
