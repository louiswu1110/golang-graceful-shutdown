package gracefulshutdown

import "context"

type funcComponent struct {
	shutdown func(context.Context) error
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
	<-ctx.Done()
	return nil
}

func (c *funcComponent) Shutdown(ctx context.Context) error {
	if c.shutdown == nil {
		return nil
	}
	return c.shutdown(ctx)
}
