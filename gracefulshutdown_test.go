package gracefulshutdown

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type testComponent struct {
	start    func(context.Context) error
	shutdown func(context.Context) error
}

func (c testComponent) Start(ctx context.Context) error    { return c.start(ctx) }
func (c testComponent) Shutdown(ctx context.Context) error { return c.shutdown(ctx) }

func blockingComponent(shutdown func(context.Context) error) Component {
	return testComponent{
		start:    func(ctx context.Context) error { <-ctx.Done(); return nil },
		shutdown: shutdown,
	}
}

func TestSequentialShutdownUsesReverseRegistrationOrder(t *testing.T) {
	var order []string
	manager := New()
	for _, name := range []string{"server", "workers", "database"} {
		name := name
		manager.Add(blockingComponent(func(context.Context) error {
			order = append(order, name)
			return nil
		}), WithName(name))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := manager.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if want := []string{"database", "workers", "server"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("shutdown order = %v, want %v", order, want)
	}
}

func TestParallelShutdown(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	manager := New(WithShutdownMode(ShutdownParallel))
	for range 2 {
		manager.Add(blockingComponent(func(context.Context) error {
			started <- struct{}{}
			<-release
			return nil
		}))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("shutdown callbacks did not run in parallel")
		}
	}
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestShutdownTimeoutIsEnforced(t *testing.T) {
	manager := New(WithTimeout(20 * time.Millisecond))
	manager.Add(blockingComponent(func(context.Context) error { select {} }))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	err := manager.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("timeout was not enforced; Run took %s", elapsed)
	}
}

func TestComponentFailureTriggersShutdown(t *testing.T) {
	want := errors.New("worker failed")
	shutdownCalled := false
	manager := New()
	manager.Add(testComponent{
		start:    func(context.Context) error { return want },
		shutdown: func(context.Context) error { shutdownCalled = true; return nil },
	}, WithName("worker"))

	err := manager.Run(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("Run() error = %v, want %v", err, want)
	}
	if !shutdownCalled {
		t.Fatal("Shutdown was not called")
	}
}

func TestCleanupAndConfigurationValidation(t *testing.T) {
	var mu sync.Mutex
	called := false
	manager := New(WithTimeout(0))
	manager.Add(CleanupFunc(func() error {
		mu.Lock()
		defer mu.Unlock()
		called = true
		return nil
	}))
	if err := manager.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want invalid timeout error")
	}
	mu.Lock()
	defer mu.Unlock()
	if called {
		t.Fatal("component started despite invalid configuration")
	}
}
