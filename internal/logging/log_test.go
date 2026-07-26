package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogPath(t *testing.T) {
	p := logPath()
	if !strings.HasSuffix(p, "heartbeat.log") {
		t.Errorf("logPath() = %q, want ...heartbeat.log", p)
	}
}

func testHomeDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestWrite(t *testing.T) {
	home := testHomeDir(t)
	logFile := filepath.Join(home, ".config", "oc-vpn", "heartbeat.log")

	Write("test entry one")
	Write("test entry two")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for _, line := range lines {
		if !strings.HasPrefix(line, "[") {
			t.Errorf("line %q does not start with timestamp", line)
		}
	}

	if !strings.Contains(lines[0], "test entry one") {
		t.Errorf("line 0 missing 'test entry one': %q", lines[0])
	}
	if !strings.Contains(lines[1], "test entry two") {
		t.Errorf("line 1 missing 'test entry two': %q", lines[1])
	}
}

func TestWriteAppends(t *testing.T) {
	home := testHomeDir(t)
	logFile := filepath.Join(home, ".config", "oc-vpn", "heartbeat.log")

	Write("first")
	Write("second")
	Write("third")

	data, _ := os.ReadFile(logFile)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines after 3 writes, got %d", len(lines))
	}
}

func TestWriteEmptyEntry(t *testing.T) {
	_ = testHomeDir(t)

	Write("")

	data, _ := os.ReadFile(logPath())
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least 1 line")
	}
}
