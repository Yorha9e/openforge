package adapter

import (
	"context"
	"fmt"
	"strings"

	"openforge/internal/pipeline/port"
)

// BatchSaveMessages saves multiple messages in a single INSERT operation.
func (r *PGRepository) BatchSaveMessages(ctx context.Context, msgs []*port.DBMessage) error {
	if len(msgs) == 0 {
		return nil
	}

	// Build batch INSERT query
	query := `INSERT INTO conversation_message (pipeline_id, branch_id, msg_seq, role, msg_type, content, token_count)
VALUES `

	args := make([]interface{}, 0, len(msgs)*7)
	valueStrings := make([]string, 0, len(msgs))

	for i, msg := range msgs {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*7+1, i*7+2, i*7+3, i*7+4, i*7+5, i*7+6, i*7+7))
		args = append(args, msg.PipelineID, msg.BranchID, msg.MsgSeq, msg.Role, msg.MsgType, msg.Content, msg.TokenCount)
	}

	query += strings.Join(valueStrings, ", ")
	query += ` ON CONFLICT (pipeline_id, branch_id, msg_seq) DO UPDATE SET
		content = EXCLUDED.content,
		token_count = EXCLUDED.token_count`

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}
