package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/research/specter-reimplementation/internal/researchjournal"
	"github.com/research/specter-reimplementation/internal/researchscenario"
)

type ResearchScenarioRun struct {
	ID        string                  `json:"id"`
	Actor     string                  `json:"actor"`
	CreatedAt time.Time               `json:"createdAt"`
	Report    researchscenario.Report `json:"report"`
}

func (s *Store) SaveResearchScenarioRun(ctx context.Context, id, actor string, report researchscenario.Report) (ResearchScenarioRun, error) {
	result := ResearchScenarioRun{ID: id, Actor: actor, Report: report}
	encoded, err := json.Marshal(report)
	if err != nil {
		return ResearchScenarioRun{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ResearchScenarioRun{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO app_research_scenario_runs(id, actor, report)
        VALUES ($1, $2, $3::jsonb) RETURNING created_at`, id, actor, string(encoded)).Scan(&result.CreatedAt)
	if err != nil {
		return ResearchScenarioRun{}, err
	}
	for _, entry := range researchjournal.FromReport(report) {
		if _, err := tx.Exec(ctx, `INSERT INTO app_research_journal(run_id, ordinal, kind, code, sequence, passed)
            VALUES ($1,$2,$3,$4,$5,$6)`, id, entry.Ordinal, entry.Kind, entry.Code, entry.Sequence, entry.Passed); err != nil {
			return ResearchScenarioRun{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ResearchScenarioRun{}, err
	}
	return result, nil
}

// The history is bounded at the API; older runs are retained in PostgreSQL.
func (s *Store) ResearchScenarioRuns(ctx context.Context) ([]ResearchScenarioRun, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, actor, created_at, report
		FROM app_research_scenario_runs ORDER BY created_at DESC, id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ResearchScenarioRun, 0)
	for rows.Next() {
		var run ResearchScenarioRun
		var encoded []byte
		if err := rows.Scan(&run.ID, &run.Actor, &run.CreatedAt, &encoded); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(encoded, &run.Report); err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	return result, rows.Err()
}
