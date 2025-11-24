package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andressep95/aws-backup-bridge/signer-service/internal/config"
	"github.com/andressep95/aws-backup-bridge/signer-service/internal/handler"
	"github.com/andressep95/aws-backup-bridge/signer-service/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting signer-service on port %s", cfg.Port)
	log.Printf("AWS Region: %s", cfg.AWSRegion)
	log.Printf("S3 Bucket: %s", cfg.S3BucketName)
	log.Printf("Presigned URL Expiration: %d minutes", cfg.PresignedURLExpirationMinutes)

	// Initialize S3 service
	s3Service, err := service.NewS3Service(cfg)
	if err != nil {
		log.Fatalf("Failed to create S3 service: %v", err)
	}

	// Initialize handlers
	h := handler.NewHandler(s3Service)

	// Setup routes
	router := h.SetupRoutes()

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
		log.Printf("Server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
