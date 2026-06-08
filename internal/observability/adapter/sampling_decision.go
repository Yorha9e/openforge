package adapter

import (
	"hash/fnv"
	"log/slog"
)

// Decision is the result of a sampling evaluation.
type Decision int

const (
	// Drop means the log entry should not be emitted.
	Drop Decision = iota
	// Keep means the log entry should be emitted.
	Keep
)

// infoSampleModulus implements a deterministic ~10% sampling rate for INFO
// level messages using FNV-1a hash modulo 10. 0..9, keep when hash%10 == 0.
const infoSampleModulus = 10

// ShouldSample decides whether a log entry at the given level with the given
// message should be emitted.
//
// Rules:
//   - WARN and above are always KEEP (audit/operational visibility).
//   - INFO uses a deterministic 10% sample based on the FNV-1a hash of msg.
//   - DEBUG and below are always DROP.
func ShouldSample(level slog.Level, msg string) Decision {
	switch {
	case level >= slog.LevelWarn:
		return Keep
	case level == slog.LevelInfo:
		h := fnv.New32a()
		_, _ = h.Write([]byte(msg))
		if h.Sum32()%infoSampleModulus == 0 {
			return Keep
		}
		return Drop
	default:
		return Drop
	}
}
