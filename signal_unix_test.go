//go:build unix

package gracefulshutdown

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestSIGTERMTriggersShutdown(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	manager := New(WithTimeout(time.Second)).Add(testComponent{
		start: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return nil
		},
		shutdown: func(context.Context) error {
			close(stopped)
			return nil
		},
	})
	done := make(chan error, 1)
	go func() { done <- manager.Run(context.Background()) }()
	<-started
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil for signal shutdown", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after SIGTERM")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("Shutdown was not called")
	}
}
