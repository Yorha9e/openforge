package domain

import "time"

// CapacityLevel represents system load water level.
type CapacityLevel int

const (
	CapacityNormal   CapacityLevel = iota // C > 30%
	CapacityWarning                       // C 10-30%
	CapacityCritical                      // C < 10%
)

func (c CapacityLevel) String() string {
	switch c {
	case CapacityNormal:
		return "NORMAL"
	case CapacityWarning:
		return "WARNING"
	case CapacityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// AcceptsPriority returns true if a request with the given priority (0 = highest)
// is accepted at this capacity level.
//   NORMAL:  all priorities
//   WARNING: P0-P2 (priority <= 2)
//   CRITICAL: P0 only (priority == 0)
func (c CapacityLevel) AcceptsPriority(priority int) bool {
	switch c {
	case CapacityNormal:
		return true
	case CapacityWarning:
		return priority <= 2
	case CapacityCritical:
		return priority == 0
	default:
		return false
	}
}

// RetryAfter returns the suggested back-off duration for rejected requests.
func (c CapacityLevel) RetryAfter() time.Duration {
	switch c {
	case CapacityNormal:
		return 0
	case CapacityWarning:
		return 5 * time.Second
	case CapacityCritical:
		return 30 * time.Second
	default:
		return time.Minute
	}
}

// ResourceSnapshot captures component signals used to compute capacity.
type ResourceSnapshot struct {
	GoroutinesAvail   int
	GoroutinesMax     int
	SandboxWarm       int
	SandboxMin        int
	PGIdleConns       int
	LLMQueueDepth     int
	LLMQueueThreshold int
}

// ShedDecision is the result of a Shed call.
type ShedDecision struct {
	Accept     bool
	RetryAfter time.Duration
	Level      CapacityLevel
}

// LoadShedder computes system capacity and decides whether to accept or reject
// a request based on water level and priority.
type LoadShedder struct{}

func NewLoadShedder() *LoadShedder {
	return &LoadShedder{}
}

// ComputeCapacity calculates available capacity from component signals.
// C = min(goroutines_avail/max, sandbox_warm/min, llm_queue_depth < threshold,
//
//	pg_idle_conns > 5)
func ComputeCapacity(s ResourceSnapshot) float64 {
	if s.GoroutinesMax == 0 || s.SandboxMin == 0 || s.LLMQueueThreshold == 0 {
		return 0
	}
	factors := []float64{
		float64(s.GoroutinesAvail) / float64(s.GoroutinesMax),
		float64(s.SandboxWarm) / float64(s.SandboxMin),
	}
	if s.LLMQueueDepth >= s.LLMQueueThreshold {
		factors = append(factors, 0)
	} else {
		factors = append(factors, 1.0)
	}
	if s.PGIdleConns <= 5 {
		factors = append(factors, 0)
	} else {
		factors = append(factors, 1.0)
	}
	min := factors[0]
	for _, f := range factors[1:] {
		if f < min {
			min = f
		}
	}
	return min * 100
}

// GetCapacityLevel returns the water level for a given capacity percentage.
func GetCapacityLevel(pct float64) CapacityLevel {
	switch {
	case pct > 30:
		return CapacityNormal
	case pct >= 10:
		return CapacityWarning
	default:
		return CapacityCritical
	}
}

// Shed evaluates a request against current resource snapshot and priority,
// returning a ShedDecision.
func (ls *LoadShedder) Shed(snapshot ResourceSnapshot, priority int) ShedDecision {
	pct := ComputeCapacity(snapshot)
	level := GetCapacityLevel(pct)
	accept := level.AcceptsPriority(priority)
	return ShedDecision{
		Accept:     accept,
		RetryAfter: level.RetryAfter(),
		Level:      level,
	}
}
