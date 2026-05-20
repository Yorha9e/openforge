package domain

import "testing"

func TestClassifyComplexity(t *testing.T) {
	tests := []struct {
		name    string
		files   int
		modules int
		want    string
	}{
		{"typo fix", 1, 1, "L1"},
		{"small feature", 3, 2, "L2"},
		{"new module", 6, 4, "L3"},
		{"refactor", 10, 5, "L4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyComplexity(tt.files, tt.modules)
			if got != tt.want {
				t.Errorf("ClassifyComplexity(%d, %d) = %q, want %q", tt.files, tt.modules, got, tt.want)
			}
		})
	}
}
