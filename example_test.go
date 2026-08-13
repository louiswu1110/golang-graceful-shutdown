package gracefulshutdown_test

import (
	"context"
	"net/http"
	"time"

	gracefulshutdown "github.com/louiswu1110/golang-graceful-shutdown"
)

func Example() {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           http.NewServeMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	manager := gracefulshutdown.New(gracefulshutdown.WithTimeout(30 * time.Second))
	manager.Add(
		gracefulshutdown.HTTPServer(server),
		gracefulshutdown.WithName("api"),
	)

	// Run normally blocks until SIGINT, SIGTERM, context cancellation, or until
	// a component exits.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = manager.Run(ctx)
}

func ExampleComponentFuncs() {
	manager := gracefulshutdown.New()
	manager.Add(gracefulshutdown.ComponentFuncs(
		func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
		func(context.Context) error {
			// Stop accepting work and wait for active jobs here.
			return nil
		},
	), gracefulshutdown.WithName("workers"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = manager.Run(ctx)
}
