package gracefulshutdown

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestNilHTTPServerReturnsErrors(t *testing.T) {
	component := HTTPServer(nil)
	if err := component.Start(context.Background()); err == nil {
		t.Fatal("Start() error = nil")
	}
	if err := component.Shutdown(context.Background()); err == nil {
		t.Fatal("Shutdown() error = nil")
	}
}

func TestHTTPServerNormalizesServerClosed(t *testing.T) {
	server := &http.Server{Addr: "127.0.0.1:0"}
	component := HTTPServer(server)
	done := make(chan error, 1)
	go func() { done <- component.Start(context.Background()) }()
	if err := component.Shutdown(context.Background()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
