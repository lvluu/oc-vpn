// Package logging provides a simple file-based logger for heartbeat and debug output.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lvluu/oc-vpn/internal/config"
)

func logPath() string {
	return filepath.Join(config.Dir(), "heartbeat.log")
}

func Write(entry string) {
	p := logPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(f, "[%s] %s\n", time.Now().Format(time.RFC3339), entry)
	_ = f.Close()
}
