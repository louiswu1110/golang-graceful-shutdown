package gracefulshutdown

import "context"

type funcComponent struct {
	start    func(context.Context) error
	shutdown func(context.Context) error
}

// ComponentFuncs creates a Component from start and shutdown functions. A nil
// start function waits until shutdown; a nil shutdown function is a no-op.
func ComponentFuncs(start, shutdown func(context.Context) error) Component {
	return &funcComponent{start: start, shutdown: shutdown}
}

// Cleanup adapts a shutdown-only function to Component. It remains running
// until the Manager begins shutdown.
func Cleanup(fn func(context.Context) error) Component {
	return &funcComponent{shutdown: fn}
}

// CleanupFunc adapts a context-free cleanup function to Component.
func CleanupFunc(fn func() error) Component {
	if fn == nil {
		return &funcComponent{}
	}
	return Cleanup(func(context.Context) error { return fn() })
}

func (c *funcComponent) Start(ctx context.Context) error {
	if c.start != nil {
		return c.start(ctx)
	}
	<-ctx.Done()
	return nil
}

func (c *funcComponent) Shutdown(ctx context.Context) error {
	if c.shutdown == nil {
		return nil
	}
	return c.shutdown(ctx)
}
