package featureflags

import "sync"

// FeatureFlags groups enterprise capabilities into 4 toggleable switches.
// Each flag controls a cluster of related Phase 9-10 features.
// G1 FIX: embedded sync.RWMutex for HTTP handler + goroutine concurrency safety.
type FeatureFlags struct {
	mu                     sync.RWMutex
	EnterprisePlatform     bool `json:"enterprise_platform" yaml:"enterprise_platform"`
	ComplianceSuite        bool `json:"compliance_suite" yaml:"compliance_suite"`
	ProductionOps          bool `json:"production_ops" yaml:"production_ops"`
	DistributionArtifacts  bool `json:"distribution_artifacts" yaml:"distribution_artifacts"`
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

// Clone returns a deep copy.
func (f *FeatureFlags) Clone() *FeatureFlags {
	c := *f
	return &c
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
