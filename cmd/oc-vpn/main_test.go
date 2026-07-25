package main

import (
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCmd  string
		wantArgs []string
	}{
		{
			name:     "import with name",
			args:     []string{"import", "config.conf", "-n", "us-east"},
			wantCmd:  "import",
			wantArgs: []string{"config.conf", "-n", "us-east"},
		},
		{
			name:     "up with profile",
			args:     []string{"up", "vpnbook-us"},
			wantCmd:  "up",
			wantArgs: []string{"vpnbook-us"},
		},
		{
			name:     "run with name and command",
			args:     []string{"run", "us-east", "curl", "ifconfig.me"},
			wantCmd:  "run",
			wantArgs: []string{"us-east", "curl", "ifconfig.me"},
		},
		{
			name:     "list",
			args:     []string{"list"},
			wantCmd:  "list",
			wantArgs: []string{},
		},
		{
			name:     "doctor",
			args:     []string{"doctor"},
			wantCmd:  "doctor",
			wantArgs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.args[0]
			args := tt.args[1:]
			if cmd != tt.wantCmd {
				t.Errorf("cmd = %q, want %q", cmd, tt.wantCmd)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args len = %d, want %d", len(args), len(tt.wantArgs))
				return
			}
			for i, a := range args {
				if a != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, a, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestNeedRoot(t *testing.T) {
	needRoot := map[string]bool{
		"up": true, "down": true, "shell": true,
		"import": true, "export": true, "remove": true,
		"list": false, "status": false, "doctor": false,
	}

	for cmd, expected := range needRoot {
		if got := needRoot[cmd]; got != expected {
			t.Errorf("needRoot[%s] = %v, want %v", cmd, got, expected)
		}
	}
}
