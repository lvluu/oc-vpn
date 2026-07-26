package wireguard

import (
	"testing"
	"time"
)

func TestWatchdogStatusDefault(t *testing.T) {
	info := WatchdogStatus("nonexistent-profile")
	if info == nil {
		t.Fatal("WatchdogStatus() returned nil")
	}
	if info.Active {
		t.Error("WatchdogStatus() for unknown profile should be inactive")
	}
}

func TestWatchdogStop(t *testing.T) {
	stop := Watchdog("test-stop", false)
	defer func() {
		watchdogStates.Delete("test-stop")
	}()

	info := WatchdogStatus("test-stop")
	info.mu.Lock()
	active := info.Active
	info.mu.Unlock()
	if !active {
		t.Error("WatchdogStatus should show active after Watchdog()")
	}

	stop()

	time.Sleep(50 * time.Millisecond)

	info = WatchdogStatus("test-stop")
	info.mu.Lock()
	active = info.Active
	info.mu.Unlock()
	if active {
		t.Error("WatchdogStatus should show inactive after stop")
	}
}

func TestWatchdogInfoTracking(t *testing.T) {
	stop := Watchdog("test-info", false)
	defer func() {
		stop()
		watchdogStates.Delete("test-info")
	}()

	info := WatchdogStatus("test-info")
	if !info.Active {
		t.Fatal("expected active watchdog")
	}

	info.mu.Lock()
	info.LastOK = true
	info.Failures = 0
	info.LastMessage = "OK handshake=10s latency=50ms"
	info.LastCheck = time.Now()
	info.mu.Unlock()

	info2 := WatchdogStatus("test-info")
	info2.mu.Lock()
	gotOK := info2.LastOK
	gotMsg := info2.LastMessage
	info2.mu.Unlock()
	if !gotOK {
		t.Error("expected LastOK=true")
	}
	if gotMsg != "OK handshake=10s latency=50ms" {
		t.Errorf("LastMessage = %q, want %q", gotMsg, "OK handshake=10s latency=50ms")
	}
}

func TestWatchdogMultiple(t *testing.T) {
	stop1 := Watchdog("multi-a", false)
	stop2 := Watchdog("multi-b", false)
	defer func() {
		stop1()
		stop2()
		watchdogStates.Delete("multi-a")
		watchdogStates.Delete("multi-b")
	}()

	checkActive := func(prof string) bool {
		inf := WatchdogStatus(prof)
		inf.mu.Lock()
		v := inf.Active
		inf.mu.Unlock()
		return v
	}

	if !checkActive("multi-a") {
		t.Error("multi-a should be active")
	}
	if !checkActive("multi-b") {
		t.Error("multi-b should be active")
	}

	stop1()

	time.Sleep(50 * time.Millisecond)

	if checkActive("multi-a") {
		t.Error("multi-a should be inactive after stop")
	}
	if !checkActive("multi-b") {
		t.Error("multi-b should still be active")
	}
}
