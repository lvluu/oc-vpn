package wireguard

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lvluu/oc-vpn/internal/logging"
	"github.com/lvluu/oc-vpn/internal/namespace"
)

const (
	checkInterval = 30 * time.Second
	maxFailures   = 3
)

type WatchdogInfo struct { //nolint:govet // field ordering not hot enough to restructure
	mu          sync.Mutex
	LastCheck   time.Time
	LastMessage string
	Failures    int
	Active      bool
	LastOK      bool
}

var watchdogStates sync.Map

func WatchdogStatus(name string) *WatchdogInfo {
	if v, ok := watchdogStates.Load(name); ok {
		return v.(*WatchdogInfo)
	}
	return &WatchdogInfo{Active: false}
}

func Watchdog(name string, stopOnFailure bool) func() {
	done := make(chan struct{})

	info := &WatchdogInfo{Active: true}
	watchdogStates.Store(name, info)

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		failures := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
			}

			if !namespace.Exists(name) {
				info.mu.Lock()
				info.LastCheck = time.Now()
				info.LastOK = false
				info.LastMessage = "ns gone"
				info.mu.Unlock()
				logging.Write(fmt.Sprintf("%s: ns gone, stopping watchdog", name))
				return
			}

			if !IsUp(name) {
				failures++
				info.mu.Lock()
				info.LastCheck = time.Now()
				info.LastOK = false
				info.Failures = failures
				info.LastMessage = fmt.Sprintf("wg0 missing (fail %d/%d)", failures, maxFailures)
				info.mu.Unlock()
				logging.Write(fmt.Sprintf("%s: %s", name, info.LastMessage))
				if stopOnFailure && failures >= maxFailures {
					logging.Write(fmt.Sprintf("%s: tearing down — wg0 gone", name))
					Down(name)
				}
				continue
			}

			out, _ := namespace.Exec(name, "wg", "show", "wg0", "latest-handshakes")
			hs := parseHandshake(out)
			age := time.Now().Unix() - hs
			hsStr := fmt.Sprintf("%ds", age)
			if hs == 0 {
				hsStr = "never"
			}

			if age > 180 && hs > 0 {
				failures++
				info.mu.Lock()
				info.LastCheck = time.Now()
				info.LastOK = false
				info.Failures = failures
				info.LastMessage = fmt.Sprintf("stale handshake %s (fail %d/%d)", hsStr, failures, maxFailures)
				info.mu.Unlock()
				logging.Write(fmt.Sprintf("%s: %s", name, info.LastMessage))
				if stopOnFailure && failures >= maxFailures {
					logging.Write(fmt.Sprintf("%s: tearing down — stale handshake", name))
					Down(name)
				}
				continue
			}

			out, err := namespace.Exec(name, "ping", "-c", "1", "-W", "3", "1.1.1.1")
			if err != nil {
				failures++
				errMsg := strings.TrimSpace(out)
				if errMsg == "" {
					errMsg = err.Error()
				}
				info.mu.Lock()
				info.LastCheck = time.Now()
				info.LastOK = false
				info.Failures = failures
				info.LastMessage = fmt.Sprintf("ping fail: %s (fail %d/%d)", errMsg, failures, maxFailures)
				info.mu.Unlock()
				logging.Write(fmt.Sprintf("%s: %s", name, info.LastMessage))
				if stopOnFailure && failures >= maxFailures {
					logging.Write(fmt.Sprintf("%s: tearing down — ping failure", name))
					Down(name)
				}
				continue
			}

			failures = 0
			info.mu.Lock()
			info.LastCheck = time.Now()
			info.LastOK = true
			info.Failures = 0
			info.LastMessage = fmt.Sprintf("OK handshake=%s latency=%s", hsStr, Latency(name))
			info.mu.Unlock()
			logging.Write(fmt.Sprintf("%s: %s", name, info.LastMessage))
		}
	}()

	var closeOnce sync.Once
	return func() {
		closeOnce.Do(func() {
			close(done)
			info.mu.Lock()
			info.Active = false
			info.mu.Unlock()
			watchdogStates.Delete(name)
			logging.Write(fmt.Sprintf("%s: watchdog stopped", name))
		})
	}
}
