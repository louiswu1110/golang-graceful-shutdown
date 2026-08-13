# gracefulshutdown

[繁體中文](README.zh-TW.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/louiswu1110/golang-graceful-shutdown.svg)](https://pkg.go.dev/github.com/louiswu1110/golang-graceful-shutdown)
[![Go](https://github.com/louiswu1110/golang-graceful-shutdown/actions/workflows/go.yml/badge.svg)](https://github.com/louiswu1110/golang-graceful-shutdown/actions/workflows/go.yml)

`gracefulshutdown` is a small lifecycle coordinator for Go services. It starts
HTTP servers, worker pools, background jobs, and custom components together,
then shuts them down when:

- the parent context is canceled;
- `SIGINT` or `SIGTERM` is received;
- any component stops or returns an error.

The package uses only the Go standard library. Go 1.23 or newer is required.

## Install

```bash
go get github.com/louiswu1110/golang-graceful-shutdown
```

## Quick start

```go
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	gracefulshutdown "github.com/louiswu1110/golang-graceful-shutdown"
)

func main() {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	manager := gracefulshutdown.New(
		gracefulshutdown.WithTimeout(30 * time.Second),
	)
	manager.Add(
		gracefulshutdown.HTTPServer(server),
		gracefulshutdown.WithName("api"),
	)
	manager.Add(
		gracefulshutdown.CleanupFunc(func() error {
			return closeDatabase()
		}),
		gracefulshutdown.WithName("database"),
	)

	if err := manager.Run(context.Background()); err != nil {
		log.Printf("service stopped: %v", err)
	}
}

func closeDatabase() error { return nil }
```

Names are optional. They make lifecycle errors easier to identify; otherwise,
the component's Go type is used.

## Gin

Gin implements `http.Handler`, so it does not need a framework-specific
adapter:

```go
router := gin.New()
server := &http.Server{Addr: ":8080", Handler: router}

manager := gracefulshutdown.New()
manager.Add(
	gracefulshutdown.HTTPServer(server),
	gracefulshutdown.WithName("gin-api"),
)
err := manager.Run(context.Background())
```

The same pattern works with Chi, Echo, Gorilla Mux, and any other
`http.Handler`.

## Custom servers and worker pools

Implement `Component` when a service has its own lifecycle:

```go
type Component interface {
	Start(context.Context) error
	Shutdown(context.Context) error
}
```

`Start` should block for as long as the component is running. `Shutdown` should
stop accepting new work, finish in-flight work, and honor its context deadline.
For small integrations, `ComponentFuncs` avoids creating a new type:

```go
manager.Add(gracefulshutdown.ComponentFuncs(
	func(ctx context.Context) error { return pool.Run(ctx) },
	func(ctx context.Context) error { return pool.Shutdown(ctx) },
), gracefulshutdown.WithName("workers"))
```

See the runnable [HTTP example](examples/http/main.go), [Gin
example](examples/gin), and [worker pool example](examples/worker_pool/main.go).

## Shutdown order

Sequential shutdown is the default. Components stop in reverse registration
order, which makes resource dependencies explicit:

```go
manager.Add(database) // stopped last
manager.Add(workers)
manager.Add(server)   // stopped first
```

To stop independent components concurrently:

```go
manager := gracefulshutdown.New(
	gracefulshutdown.WithShutdownMode(gracefulshutdown.ShutdownParallel),
)
```

`WithTimeout` applies one deadline to the complete shutdown operation. The
manager returns joined errors, so callers can use `errors.Is` and `errors.As`.

## Design notes

- Calling `Run` more than once is an error.
- A component that exits successfully still initiates shutdown; a managed
  component is expected to remain running.
- OS signals initiate a clean shutdown and are not themselves returned as
  errors. Parent-context cancellation is returned to the caller.
- A shutdown callback should honor its context. The manager enforces its own
  deadline, but Go cannot forcibly terminate a callback that ignores
  cancellation.

## Contributing

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
