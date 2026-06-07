package featureflags

import (
	"sync"
	"time"
)

// FeatureFlags groups enterprise capabilities into 4 toggleable switches.
// Each flag controls a cluster of related Phase 9-10 features.
// G1 FIX: embedded sync.RWMutex for HTTP handler + goroutine concurrency safety.
type FeatureFlags struct {
	mu                     sync.RWMutex
	EnterprisePlatform     bool `json:"enterprise_platform" yaml:"enterprise_platform"`
	ComplianceSuite        bool `json:"compliance_suite" yaml:"compliance_suite"`
	ProductionOps          bool `json:"production_ops" yaml:"production_ops"`
	DistributionArtifacts  bool `json:"distribution_artifacts" yaml:"distribution_artifacts"`

	// Lifecycle holds per-flag ownership, status, expiration, and rollout percent.
	// Keyed by the flag name (e.g. "enterprise_platform").
	Lifecycle map[string]FlagLifecycle `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
}

// FlagLifecycle captures ownership + state-machine + expiration metadata
// for a single feature flag. The Status field drives the state machine
// transitions enforced by ValidateTransition.
type FlagLifecycle struct {
	Owner      string     `json:"owner" yaml:"owner"`
	Status     string     `json:"status" yaml:"status"`
	RolloutPct int        `json:"rollout_pct" yaml:"rollout_pct"`
	CreatedAt  time.Time  `json:"created_at" yaml:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
}

// Lock/Unlock/RLock/RUnlock delegates for external use (e.g. PUT handler).
func (f *FeatureFlags) Lock()    { f.mu.Lock() }
func (f *FeatureFlags) Unlock()  { f.mu.Unlock() }
func (f *FeatureFlags) RLock()   { f.mu.RLock() }
func (f *FeatureFlags) RUnlock() { f.mu.RUnlock() }

// Defaults returns the hardcoded zero-value defaults (all false).
// Profile YAML values override these at bootstrap time.
func Defaults() *FeatureFlags {
	return &FeatureFlags{}
}

// Clone returns a deep copy (mutex is not copied — new instance gets its own).
func (f *FeatureFlags) Clone() *FeatureFlags {
	f.RLock()
	defer f.RUnlock()
	clone := &FeatureFlags{
		EnterprisePlatform:    f.EnterprisePlatform,
		ComplianceSuite:       f.ComplianceSuite,
		ProductionOps:         f.ProductionOps,
		DistributionArtifacts: f.DistributionArtifacts,
	}
	if f.Lifecycle != nil {
		clone.Lifecycle = make(map[string]FlagLifecycle, len(f.Lifecycle))
		for k, v := range f.Lifecycle {
			clone.Lifecycle[k] = v
		}
	}
	return clone
}

// AllFlags returns the 4 flag keys in canonical order.
func AllFlags() []string {
	return []string{
		"enterprise_platform",
		"compliance_suite",
		"production_ops",
		"distribution_artifacts",
	}
}
