package domain

import (
	"encoding/json"
	"time"
)

type Checkpoint struct {
	PipelineID string
	Stage      string
	Seq        int
	Data       json.RawMessage
	Trigger    string
	CreatedAt  time.Time
}

const MaxInMemoryCheckpoints = 3
