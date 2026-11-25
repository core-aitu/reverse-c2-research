package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/research/specter-reimplementation/internal/domain"
)

func (s *Store) AiAgents(ctx context.Context) ([]domain.AiAgentDefinition, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.id,a.agentid,a.name,a.description,a.prompt,a.includespecterscripts,
		COALESCE(array_agg(f.funcid ORDER BY f.funcid) FILTER (WHERE f.funcid IS NOT NULL),'{}')
		FROM aiagentdefinitions a
		LEFT JOIN aiagentfunctionlinks l ON l.agentid=a.id
		LEFT JOIN aifunctiondefinitions f ON f.id=l.functionid
		GROUP BY a.id ORDER BY a.name,a.agentid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AiAgentDefinition{}
	for rows.Next() {
		var value domain.AiAgentDefinition
		if err := rows.Scan(&value.ID, &value.AgentID, &value.Name, &value.Description, &value.Prompt,
			&value.IncludeSpecterScripts, &value.AccessibleFunctions); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) PutAiAgent(ctx context.Context, originalID string, value domain.AiAgentDefinition) error {
	if originalID != "" {
		value.AgentID = originalID
	}
	if err := validateAiAgent(value); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var databaseID int64
	err = tx.QueryRow(ctx, `INSERT INTO aiagentdefinitions(agentid,name,description,prompt,includespecterscripts)
		VALUES($1,$2,$3,$4,$5) ON CONFLICT(agentid) DO UPDATE SET name=excluded.name,
		description=excluded.description,prompt=excluded.prompt,includespecterscripts=excluded.includespecterscripts
		RETURNING id`, value.AgentID, value.Name, value.Description, value.Prompt, value.IncludeSpecterScripts).Scan(&databaseID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM aiagentfunctionlinks WHERE agentid=$1", databaseID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, functionID := range value.AccessibleFunctions {
		functionID = strings.TrimSpace(functionID)
		if functionID == "" || seen[functionID] {
			continue
		}
		seen[functionID] = true
		command, err := tx.Exec(ctx, `INSERT INTO aiagentfunctionlinks(agentid,functionid)
			SELECT $1,id FROM aifunctiondefinitions WHERE funcid=$2`, databaseID, functionID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return errors.New("accessible AI function not found: " + functionID)
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteAiAgent(ctx context.Context, agentID string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM aiagentdefinitions WHERE agentid=$1", agentID)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func validateAiAgent(value domain.AiAgentDefinition) error {
	if !validCatalogID(value.AgentID) || strings.TrimSpace(value.Name) == "" || len(value.Name) > 256 ||
		len(value.Description) > 4096 || len(value.Prompt) > 1<<20 || len(value.AccessibleFunctions) > 128 {
		return errors.New("invalid AI agent definition")
	}
	for _, functionID := range value.AccessibleFunctions {
		if !validCatalogID(functionID) {
			return errors.New("invalid accessible AI function id")
		}
	}
	return nil
}

func (s *Store) AiFunctions(ctx context.Context) ([]domain.AiFunctionDefinition, error) {
	rows, err := s.pool.Query(ctx, `SELECT funcid,name,description,documentation,approvallevel::text,definition
		FROM aifunctiondefinitions ORDER BY name,funcid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AiFunctionDefinition{}
	for rows.Next() {
		var value domain.AiFunctionDefinition
		if err := rows.Scan(&value.FunctionID, &value.Name, &value.Description, &value.Documentation,
			&value.ApprovalLevel, &value.Definition); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) AiFunction(ctx context.Context, functionID string) (domain.AiFunctionDefinition, error) {
	var value domain.AiFunctionDefinition
	err := s.pool.QueryRow(ctx, `SELECT funcid,name,description,documentation,approvallevel::text,definition
		FROM aifunctiondefinitions WHERE funcid=$1`, functionID).Scan(&value.FunctionID, &value.Name,
		&value.Description, &value.Documentation, &value.ApprovalLevel, &value.Definition)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AiFunctionDefinition{}, ErrNotFound
	}
	return value, err
}

func (s *Store) PutAiFunction(ctx context.Context, originalID, internalName string, value domain.AiFunctionDefinition) error {
	if originalID != "" {
		value.FunctionID = originalID
	}
	if err := validateAiFunction(value); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO aifunctiondefinitions(funcid,name,internalname,description,documentation,definition,approvallevel)
		VALUES($1,$2,$3,$4,$5,$6,$7::toolcallapprovallevel)
		ON CONFLICT(funcid) DO UPDATE SET name=excluded.name,internalname=excluded.internalname,
		description=excluded.description,documentation=excluded.documentation,definition=excluded.definition,
		approvallevel=excluded.approvallevel`, value.FunctionID, value.Name, internalName, value.Description,
		value.Documentation, value.Definition, value.ApprovalLevel)
	return err
}

func (s *Store) DeleteAiFunction(ctx context.Context, functionID string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM aifunctiondefinitions WHERE funcid=$1", functionID)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func validateAiFunction(value domain.AiFunctionDefinition) error {
	if !validCatalogID(value.FunctionID) || strings.TrimSpace(value.Name) == "" || len(value.Name) > 256 ||
		len(value.Description) > 4096 || len(value.Documentation) > 1<<20 || len(value.Definition) > 64<<10 ||
		(value.ApprovalLevel != "Low" && value.ApprovalLevel != "Medium" && value.ApprovalLevel != "High") {
		return errors.New("invalid AI function definition")
	}
	return nil
}

func validCatalogID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if !(character == '-' || character == '_' || character == '.' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}
