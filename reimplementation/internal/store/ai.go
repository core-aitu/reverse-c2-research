package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

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

func (s *Store) CreateAiSession(ctx context.Context, value domain.AiSessionInfo) (string, error) {
	if err := s.validateAiSession(ctx, value); err != nil {
		return "", err
	}
	agents, _ := json.Marshal(value.Agents)
	plan := value.Plan
	if len(plan) == 0 {
		plan = json.RawMessage(`{}`)
	}
	for attempt := 0; attempt < 3; attempt++ {
		sessionID := randomStoreID("ai-session")
		_, err := s.pool.Exec(ctx, `INSERT INTO aisessions(sessionid,name,agents,modeltype,modelname,approvalpolicy,status,plan)
			VALUES($1,$2,$3,$4,$5,$6::toolcallapprovalpolicy,$7::aisessionstatus,$8)`, sessionID, value.Name,
			string(agents), value.ModelType, value.ModelName, value.ToolApprovalPolicy, value.Status, string(plan))
		if err == nil {
			return sessionID, nil
		}
		if !isUniqueViolation(err) {
			return "", err
		}
	}
	return "", errors.New("could not allocate AI session id")
}

func (s *Store) AiSessions(ctx context.Context) ([]domain.AiSessionInfoWithSessionID, error) {
	rows, err := s.pool.Query(ctx, `SELECT sessionid,name,agents,modeltype,modelname,approvalpolicy::text,status::text,plan
		FROM aisessions ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AiSessionInfoWithSessionID{}
	for rows.Next() {
		value, err := scanAiSession(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) AiSession(ctx context.Context, sessionID string) (domain.AiSessionInfoWithSessionID, error) {
	value, err := scanAiSession(s.pool.QueryRow(ctx, `SELECT sessionid,name,agents,modeltype,modelname,
		approvalpolicy::text,status::text,plan FROM aisessions WHERE sessionid=$1`, sessionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AiSessionInfoWithSessionID{}, ErrNotFound
	}
	return value, err
}

type aiSessionScanner interface{ Scan(...any) error }

func scanAiSession(row aiSessionScanner) (domain.AiSessionInfoWithSessionID, error) {
	var value domain.AiSessionInfoWithSessionID
	var agents, plan string
	err := row.Scan(&value.SessionID, &value.Name, &agents, &value.ModelType, &value.ModelName,
		&value.ToolApprovalPolicy, &value.Status, &plan)
	if err != nil {
		return domain.AiSessionInfoWithSessionID{}, err
	}
	if err := json.Unmarshal([]byte(agents), &value.Agents); err != nil {
		return domain.AiSessionInfoWithSessionID{}, err
	}
	value.Plan = json.RawMessage(plan)
	if !json.Valid(value.Plan) {
		value.Plan = json.RawMessage(`{}`)
	}
	return value, nil
}

func (s *Store) validateAiSession(ctx context.Context, value domain.AiSessionInfo) error {
	if strings.TrimSpace(value.Name) == "" || len(value.Name) > 256 || value.ModelType != "fixture" ||
		value.ModelName != "fixture-deterministic" || len(value.Agents) > 16 || len(value.Plan) > 1<<20 ||
		(value.ToolApprovalPolicy != "AutoApproveAll" && value.ToolApprovalPolicy != "AutoApproveLow" &&
			value.ToolApprovalPolicy != "AutoApproveMedium" && value.ToolApprovalPolicy != "AutoApproveNone") ||
		(value.Status != "Running" && value.Status != "Pausing" && value.Status != "Paused" &&
			value.Status != "Starting" && value.Status != "Stopped") {
		return errors.New("invalid AI session")
	}
	if len(value.Plan) > 0 && !json.Valid(value.Plan) {
		return errors.New("invalid AI session plan")
	}
	for _, agent := range value.Agents {
		var exists bool
		if err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM aiagentdefinitions WHERE agentid=$1)", agent.AgentID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errors.New("AI session agent not found: " + agent.AgentID)
		}
	}
	return nil
}

func (s *Store) DeleteAiSession(ctx context.Context, sessionID string) error {
	command, err := s.pool.Exec(ctx, "DELETE FROM aisessions WHERE sessionid=$1", sessionID)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) SetAiSessionStatus(ctx context.Context, sessionID, status string) error {
	if status != "Paused" && status != "Running" && status != "Stopped" {
		return errors.New("invalid AI session status transition")
	}
	command, err := s.pool.Exec(ctx, "UPDATE aisessions SET status=$2::aisessionstatus WHERE sessionid=$1", sessionID, status)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) AiMessages(ctx context.Context, sessionID string) ([]domain.AiMessage, error) {
	rows, err := s.pool.Query(ctx, `SELECT s.sessionid,m.messageid,m.timecreated,m.timecompleted,m.sendername,
		m.sendertype::text,m.receivername,m.message,COALESCE(m.summarizedtext,''),m.status
		FROM aimessages m JOIN aisessions s ON s.id=m.sessionid WHERE s.sessionid=$1 ORDER BY m.timecreated,m.id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AiMessage{}
	for rows.Next() {
		var value domain.AiMessage
		if err := rows.Scan(&value.SessionID, &value.MessageID, &value.TimeCreated, &value.TimeCompleted,
			&value.SenderName, &value.SenderType, &value.ReceiverName, &value.Message, &value.SummarizedText,
			&value.Status); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) AddAiMessage(ctx context.Context, value domain.AiMessage) error {
	if !validCatalogID(value.SessionID) || len(value.Message) == 0 || len(value.Message) > 1<<20 ||
		len(value.SenderName) > 256 || len(value.ReceiverName) > 256 {
		return errors.New("invalid AI message")
	}
	if value.MessageID == "" {
		value.MessageID = randomStoreID("message")
	}
	if value.SenderType == "" {
		value.SenderType = "User"
	}
	if value.SenderType != "User" {
		return errors.New("only user messages are accepted by the fixture session")
	}
	if value.Status == "" {
		value.Status = "Complete"
	}
	command, err := s.pool.Exec(ctx, `INSERT INTO aimessages(sessionid,messageid,timecreated,timecompleted,sendertype,
		sendername,receivername,message,summarizedtext,status)
		SELECT id,$2,now(),now(),$3::aisendertype,$4,$5,$6,NULLIF($7,''),$8 FROM aisessions WHERE sessionid=$1`,
		value.SessionID, value.MessageID, value.SenderType, value.SenderName, value.ReceiverName, value.Message,
		value.SummarizedText, value.Status)
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) AiFunctionResults(ctx context.Context, sessionID string) ([]domain.AiFunctionCallResult, error) {
	rows, err := s.pool.Query(ctx, `SELECT s.sessionid,r.toolcallid,r.name,r.description,r.argumentsjson,r.output,
		COALESCE(r.error,''),r.executedutc,r.approvalstatus::text FROM aifunctioncallresults r
		JOIN aisessions s ON s.id=r.sessionid WHERE s.sessionid=$1 ORDER BY r.executedutc,r.id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AiFunctionCallResult{}
	for rows.Next() {
		var value domain.AiFunctionCallResult
		if err := rows.Scan(&value.SessionID, &value.ToolCallID, &value.Name, &value.Description,
			&value.ArgumentsJSON, &value.Output, &value.Error, &value.ExecutedUTC, &value.Status); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) DecideAiFunctionCall(ctx context.Context, toolCallID string, approved bool) error {
	status := "Denied"
	if approved {
		status = "Approved"
	}
	command, err := s.pool.Exec(ctx, `UPDATE aifunctioncallresults SET approvalstatus=$2::aifunctioncallstatus,
		executedutc=$3 WHERE toolcallid=$1 AND approvalstatus IN ('Created','PendingApproval')`, toolCallID, status, time.Now().UTC())
	if err == nil && command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
