package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openchami/boot-service/internal/storage"
)

func main() {
	// Initialize storage backend
	if err := storage.InitFileBackend("./data"); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Setup router
	r := chi.NewRouter()

	// Register generated routes
	RegisterGeneratedRoutes(r)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
