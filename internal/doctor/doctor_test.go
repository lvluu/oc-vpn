package doctor

import (
	"testing"
)

func TestRun(t *testing.T) {
	checks := Run()
	if len(checks) == 0 {
		t.Fatal("Run() returned no checks")
	}

	// Verify each check has a name and status
	for _, c := range checks {
		if c.Name == "" {
			t.Error("check has empty name")
		}
		if c.Status != "ok" && c.Status != "warn" && c.Status != "fail" {
			t.Errorf("check %q has invalid status: %q", c.Name, c.Status)
		}
		if c.Message == "" {
			t.Errorf("check %q has empty message", c.Name)
		}
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name   string
		checks []Check
		want   int
	}{
		{
			name:   "all ok",
			checks: []Check{{Name: "a", Status: "ok"}, {Name: "b", Status: "ok"}},
			want:   0,
		},
		{
			name:   "one fail",
			checks: []Check{{Name: "a", Status: "ok"}, {Name: "b", Status: "fail"}},
			want:   1,
		},
		{
			name:   "mixed",
			checks: []Check{{Name: "a", Status: "ok"}, {Name: "b", Status: "warn"}, {Name: "c", Status: "fail"}},
			want:   1,
		},
		{
			name:   "two fails",
			checks: []Check{{Name: "a", Status: "fail"}, {Name: "b", Status: "fail"}},
			want:   2,
		},
		{
			name:   "warns don't count",
			checks: []Check{{Name: "a", Status: "warn"}, {Name: "b", Status: "warn"}},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Errors(tt.checks)
			if got != tt.want {
				t.Errorf("Errors() = %d, want %d", got, tt.want)
			}
		})
	}
}
