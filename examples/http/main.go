package main

import (
	"context"
	"log"
	"net/http"
	"time"

	gracefulshutdown "github.com/louiswu1110/golang-graceful-shutdown"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	manager := gracefulshutdown.New(gracefulshutdown.WithTimeout(10 * time.Second))
	manager.Add(gracefulshutdown.HTTPServer(server), gracefulshutdown.WithName("http"))
	if err := manager.Run(context.Background()); err != nil {
		log.Printf("service stopped: %v", err)
	}
}
