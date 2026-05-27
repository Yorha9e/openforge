package adapter

import (
	"context"
	"database/sql"
	"encoding/json"

	"openforge/internal/pipeline/port"
)

var _ port.GateRequestRepository = (*PGGateRequestRepository)(nil)

type PGGateRequestRepository struct {
	db *sql.DB
}

func NewPGGateRequestRepository(db *sql.DB) *PGGateRequestRepository {
	return &PGGateRequestRepository{db: db}
}

func (r *PGGateRequestRepository) CreateRequest(ctx context.Context, req *port.DBGateRequest) error {
	var resultJSON []byte
	if req.Result != "" {
		// Wrap the result string into a valid JSON value.
		resultJSON, _ = json.Marshal(req.Result)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO gate_request (pipeline_id, stage, status, requested_by, result, timeout_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, req.PipelineID, req.Stage, "pending", req.RequestedBy, resultJSON, req.TimeoutAt)
	return err
}

func (r *PGGateRequestRepository) UpdateRequestStatus(ctx context.Context, id string, status string, approvedBy string, result string) error {
	var resultJSON []byte
	if result != "" {
		resultJSON, _ = json.Marshal(result)
	}

	var approvedByArg any
	if approvedBy != "" {
		approvedByArg = approvedBy
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE gate_request SET status = $2, approved_by = $3, approved_at = NOW(), result = $4, updated_at = NOW()
		WHERE id = $1
	`, id, status, approvedByArg, resultJSON)
	return err
}

func (r *PGGateRequestRepository) GetPendingRequests(ctx context.Context) ([]*port.DBGateRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pipeline_id, stage, status, requested_by,
		       COALESCE(approved_by::text, ''),
		       approved_at,
		       COALESCE(result::text, ''),
		       timeout_at, created_at
		FROM gate_request
		WHERE status = 'pending'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*port.DBGateRequest
	for rows.Next() {
		var req port.DBGateRequest
		var approvedBy sql.NullString
		var approvedAt sql.NullTime
		var resultStr sql.NullString
		if err := rows.Scan(&req.ID, &req.PipelineID, &req.Stage, &req.Status,
			&req.RequestedBy, &approvedBy, &approvedAt, &resultStr,
			&req.TimeoutAt, &req.CreatedAt); err != nil {
			return nil, err
		}
		if approvedBy.Valid {
			req.ApprovedBy = &approvedBy.String
		}
		if approvedAt.Valid {
			req.ApprovedAt = &approvedAt.Time
		}
		if resultStr.Valid {
			req.Result = resultStr.String
		}
		list = append(list, &req)
	}
	return list, nil
}

func (r *PGGateRequestRepository) GetActiveRequest(ctx context.Context, pipelineID, stage string) (*port.DBGateRequest, error) {
	var req port.DBGateRequest
	var approvedBy sql.NullString
	var approvedAt sql.NullTime
	var resultStr sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, pipeline_id, stage, status, requested_by,
		       COALESCE(approved_by::text, ''),
		       approved_at,
		       COALESCE(result::text, ''),
		       timeout_at, created_at
		FROM gate_request
		WHERE pipeline_id = $1 AND stage = $2 AND status = 'pending'
	`, pipelineID, stage).Scan(&req.ID, &req.PipelineID, &req.Stage, &req.Status,
		&req.RequestedBy, &approvedBy, &approvedAt, &resultStr,
		&req.TimeoutAt, &req.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if approvedBy.Valid {
		req.ApprovedBy = &approvedBy.String
	}
	if approvedAt.Valid {
		req.ApprovedAt = &approvedAt.Time
	}
	if resultStr.Valid {
		req.Result = resultStr.String
	}
	return &req, err
}

// HandleTimeouts atomically marks all timed-out pending requests as 'timeout'
// and returns the affected pipeline IDs for further cancellation.
func (r *PGGateRequestRepository) HandleTimeouts(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		UPDATE gate_request
		SET status = 'timeout', updated_at = NOW()
		WHERE status = 'pending' AND timeout_at < NOW()
		RETURNING pipeline_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pipelineIDs []string
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			return nil, err
		}
		pipelineIDs = append(pipelineIDs, pid)
	}
	return pipelineIDs, nil
}
