package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hys/backend/internal/allow"
	"github.com/hys/backend/internal/handlers"
	transport "github.com/hys/backend/internal/http"
	"github.com/hys/backend/internal/repo"
)

func main() {
	url := strings.TrimSpace(os.Getenv("PERSONNEL_XML_URL"))
	if url == "" {
		log.Fatal("PERSONNEL_XML_URL must be set")
	}

	fixture := strings.TrimSpace(os.Getenv("PERSONNEL_FIXTURE"))
	port := strings.TrimSpace(os.Getenv("API_PORT"))
	if port == "" {
		port = "8080"
	}

	client := &http.Client{Timeout: 10 * time.Second}

	source := repo.NewSource(url, fixture, 5*time.Minute, client)
	allowlist := allow.NewAllowlist(os.Getenv("ADMIN_ALLOWLIST"))

	handler := handlers.New(source, allowlist)
	router := transport.NewRouter(handler)

	addr := ":" + port
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
