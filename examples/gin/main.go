package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gracefulshutdown "github.com/louiswu1110/golang-graceful-shutdown"
)

func main() {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	manager := gracefulshutdown.New(gracefulshutdown.WithTimeout(10 * time.Second))
	manager.Add(gracefulshutdown.HTTPServer(server), gracefulshutdown.WithName("gin-api"))

	if err := manager.Run(context.Background()); err != nil {
		log.Printf("service stopped: %v", err)
	}
}
