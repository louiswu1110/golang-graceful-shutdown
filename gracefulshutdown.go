// Package gracefulshutdown coordinates the startup and graceful shutdown of
// servers, workers, and cleanup functions in a Go application.
package gracefulshutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sync"
	"syscall"
	"time"
)

// Component is a long-running part of an application. Start should block while
// the component is running and return if it stops. Shutdown should stop it and
// honor the supplied context.
type Component interface {
	Start(context.Context) error
	Shutdown(context.Context) error
}

// ShutdownMode controls how registered components are stopped.
type ShutdownMode uint8

const (
	// ShutdownSequential stops components one at a time in reverse registration order.
	ShutdownSequential ShutdownMode = iota
	// ShutdownParallel stops all components concurrently.
	ShutdownParallel
)

// Option configures a Manager.
type Option func(*Manager) error

// WithTimeout sets the total time allowed for graceful shutdown.
func WithTimeout(timeout time.Duration) Option {
	return func(m *Manager) error {
		if timeout <= 0 {
			return errors.New("gracefulshutdown: timeout must be positive")
		}
		m.timeout = timeout
		return nil
	}
}

// WithShutdownMode selects sequential or parallel shutdown.
func WithShutdownMode(mode ShutdownMode) Option {
	return func(m *Manager) error {
		if mode != ShutdownSequential && mode != ShutdownParallel {
			return errors.New("gracefulshutdown: invalid shutdown mode")
		}
		m.mode = mode
		return nil
	}
}

// ComponentOption configures one component registration.
type ComponentOption func(*registration) error

// WithName assigns a human-readable name used in returned errors.
func WithName(name string) ComponentOption {
	return func(r *registration) error {
		if name == "" {
			return errors.New("gracefulshutdown: component name must not be empty")
		}
		r.name = name
		return nil
	}
}

type registration struct {
	name      string
	component Component
}

// Manager owns a set of application components. A Manager must not be copied.
type Manager struct {
	mu         sync.Mutex
	timeout    time.Duration
	mode       ShutdownMode
	components []registration
	running    bool
	configErr  error
}

// New creates a Manager. Invalid options are reported when Run is called.
func New(options ...Option) *Manager {
	m := &Manager{timeout: 30 * time.Second, mode: ShutdownSequential}
	for _, option := range options {
		if option == nil {
			m.configErr = errors.Join(m.configErr, errors.New("gracefulshutdown: nil option"))
			continue
		}
		m.configErr = errors.Join(m.configErr, option(m))
	}
	return m
}

// Add registers a component. It returns the Manager to allow chaining.
func (m *Manager) Add(component Component, options ...ComponentOption) *Manager {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		m.configErr = errors.Join(m.configErr, errors.New("gracefulshutdown: cannot add a component after Run starts"))
		return m
	}
	if isNil(component) {
		m.configErr = errors.Join(m.configErr, errors.New("gracefulshutdown: nil component"))
		return m
	}

	r := registration{name: reflect.TypeOf(component).String(), component: component}
	for _, option := range options {
		if option == nil {
			m.configErr = errors.Join(m.configErr, errors.New("gracefulshutdown: nil component option"))
			continue
		}
		m.configErr = errors.Join(m.configErr, option(&r))
	}
	m.components = append(m.components, r)
	return m
}

// Run starts every component and blocks until the context is canceled, an
// interrupt or termination signal arrives, or a component exits. It then stops
// all components according to the configured shutdown mode.
func (m *Manager) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("gracefulshutdown: nil context")
	}

	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return errors.New("gracefulshutdown: Manager.Run may only be called once")
	}
	m.running = true
	components := append([]registration(nil), m.components...)
	configErr := m.configErr
	timeout := m.timeout
	mode := m.mode
	m.mu.Unlock()

	if configErr != nil {
		return configErr
	}
	if len(components) == 0 {
		return errors.New("gracefulshutdown: no components registered")
	}

	signalCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	type result struct {
		name string
		err  error
	}
	results := make(chan result, len(components))
	for _, item := range components {
		item := item
		go func() { results <- result{item.name, item.component.Start(runCtx)} }()
	}

	var runErr error
	select {
	case <-signalCtx.Done():
		if err := ctx.Err(); err != nil {
			runErr = err
		}
	case result := <-results:
		if result.err != nil {
			runErr = fmt.Errorf("component %q stopped: %w", result.name, result.err)
		}
	}

	cancelRun()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), timeout)
	defer cancelShutdown()
	shutdownErr := shutdown(shutdownCtx, components, mode)
	return errors.Join(runErr, shutdownErr)
}

func shutdown(ctx context.Context, components []registration, mode ShutdownMode) error {
	if mode == ShutdownParallel {
		return shutdownParallel(ctx, components)
	}
	var result error
	for i := len(components) - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			return errors.Join(result, fmt.Errorf("graceful shutdown: %w", ctx.Err()))
		}
		errCh := make(chan error, 1)
		go func(item registration) { errCh <- item.component.Shutdown(ctx) }(components[i])
		var err error
		select {
		case err = <-errCh:
		case <-ctx.Done():
			return errors.Join(result, fmt.Errorf("graceful shutdown: %w", ctx.Err()))
		}
		if err != nil {
			result = errors.Join(result, fmt.Errorf("shutdown component %q: %w", components[i].name, err))
		}
	}
	return result
}

func shutdownParallel(ctx context.Context, components []registration) error {
	errs := make(chan error, len(components))
	for i := len(components) - 1; i >= 0; i-- {
		item := components[i]
		go func() {
			if err := item.component.Shutdown(ctx); err != nil {
				errs <- fmt.Errorf("shutdown component %q: %w", item.name, err)
			} else {
				errs <- nil
			}
		}()
	}

	var result error
	for range components {
		select {
		case err := <-errs:
			result = errors.Join(result, err)
		case <-ctx.Done():
			return errors.Join(result, fmt.Errorf("graceful shutdown: %w", ctx.Err()))
		}
	}
	return result
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
