package main

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWithPanicGuardRecovers(t *testing.T) {
	var called atomic.Bool
	withPanicGuard("test.guard", func(any) {
		called.Store(true)
	}, func() {
		panic("boom")
	})
	if !called.Load() {
		t.Fatalf("panic callback was not called")
	}
}

func TestWithPanicGuardNoPanic(t *testing.T) {
	var called atomic.Bool
	withPanicGuard("test.guard.no_panic", func(any) {
		called.Store(true)
	}, func() {})
	if called.Load() {
		t.Fatalf("panic callback should not be called")
	}
}

func TestSafeGoRecoversPanic(t *testing.T) {
	done := make(chan struct{})
	safeGo("test.safe_go.panic", func() {
		defer close(done)
		panic("boom")
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("safeGo goroutine did not finish")
	}
}

func TestSafeGoRunsNormal(t *testing.T) {
	done := make(chan struct{})
	safeGo("test.safe_go.ok", func() {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("safeGo goroutine did not run")
	}
}
