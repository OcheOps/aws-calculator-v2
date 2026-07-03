package workers

import (
	"context"
	"testing"
	"time"
)

func TestScheduler_RunsJobOnTick(t *testing.T) {
	s := NewScheduler()
	ran := make(chan struct{}, 1)
	// Every second.
	if err := s.Add("@every 1s", func(context.Context) error {
		select {
		case ran <- struct{}{}:
		default:
		}
		return nil
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s.Start(context.Background())
	defer s.Stop()
	select {
	case <-ran:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not run within 3s")
	}
}

func TestScheduler_RejectsBadCron(t *testing.T) {
	s := NewScheduler()
	if err := s.Add("not-a-cron", func(context.Context) error { return nil }); err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}
