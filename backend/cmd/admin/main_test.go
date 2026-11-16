package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForDatabaseRecoversFromColdStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	calls := 0
	err := waitForDatabase(ctx, func(context.Context) error {
		calls++
		if calls < 2 {
			return errors.New("connection refused")
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestWaitForDatabaseStopsAtDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitForDatabase(ctx, func(context.Context) error { return errors.New("connection refused") })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected result: %v", err)
	}
}
