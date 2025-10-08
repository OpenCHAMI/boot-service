package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/handlers/legacy"
)

func main() {
	// Initialize storage backend
	if err := storage.InitFileBackend("./data"); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create client for legacy handler
	bootClient, err := client.NewClient("http://localhost:8080", &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Setup router
	r := chi.NewRouter()

	// Register generated routes (modern API)
	RegisterGeneratedRoutes(r)

	// Register legacy BSS API routes
	logger := log.New(log.Writer(), "legacy: ", log.LstdFlags)
	legacyHandler := legacy.NewLegacyHandler(*bootClient, logger)
	legacyHandler.RegisterRoutes(r)

	log.Println("Server starting on :8080")
	log.Println("Modern API available at: /nodes, /bootconfigurations")
	log.Println("Legacy BSS API available at: /boot/v1/")
	log.Fatal(http.ListenAndServe(":8080", r))
}
