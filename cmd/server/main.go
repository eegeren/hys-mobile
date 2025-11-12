package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"hys-backend-go/internal/allow"
	"hys-backend-go/internal/checkin"
	"hys-backend-go/internal/devices"
	"hys-backend-go/internal/handlers"
	transport "hys-backend-go/internal/http"
	"hys-backend-go/internal/notify"
	"hys-backend-go/internal/reminder"
	"hys-backend-go/internal/repo"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := loadConfig()

	loc, err := time.LoadLocation(cfg.timezone)
	if err != nil {
		log.Fatalf("load timezone %s: %v", cfg.timezone, err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	source := repo.NewSource(cfg.personnelURL, cfg.fixturePath, 5*time.Minute, client)
	allowlist := allow.NewAllowlist(cfg.allowlistInit)

	checkins, err := checkin.NewStore(cfg.checkinPath, loc)
	if err != nil {
		log.Fatalf("checkin store init: %v", err)
	}

	deviceStore, err := devices.NewStore(cfg.devicesPath)
	if err != nil {
		log.Fatalf("device store init: %v", err)
	}

	apnsClient, err := notify.FromEnv()
	if err != nil {
		log.Fatalf("apns init: %v", err)
	}

	reminderSvc, err := reminder.NewService(source, checkins, reminder.DefaultSchedules, loc)
	if err != nil {
		log.Fatalf("reminder init: %v", err)
	}
	reminderSvc.Start(ctx)

	handler := handlers.New(source, allowlist, checkins, reminderSvc, deviceStore, apnsClient, loc)
	router := transport.NewRouter(handler)

	srv := &http.Server{
		Addr:         ":" + cfg.port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("server listening on :%s", cfg.port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	case <-ctx.Done():
		log.Printf("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	if err := checkins.Flush(); err != nil {
		log.Printf("checkin flush error: %v", err)
	}
}

type config struct {
	personnelURL  string
	fixturePath   string
	checkinPath   string
	devicesPath   string
	allowlistInit string
	port          string
	timezone      string
}

func loadConfig() config {
	url := strings.TrimSpace(os.Getenv("PERSONNEL_XML_URL"))
	if url == "" {
		log.Fatal("PERSONNEL_XML_URL must be set")
	}
	port := strings.TrimSpace(os.Getenv("API_PORT"))
	if port == "" {
		port = "8080"
	}
	checkinPath := strings.TrimSpace(os.Getenv("CHECKIN_DB_PATH"))
	if checkinPath == "" {
		checkinPath = "./data/checkins.json"
	}
	devicesPath := strings.TrimSpace(os.Getenv("DEVICES_DB_PATH"))
	if devicesPath == "" {
		devicesPath = "./var/devices.json"
	}
	tz := strings.TrimSpace(os.Getenv("TZ"))
	if tz == "" {
		tz = "Europe/Istanbul"
	}
	return config{
		personnelURL:  url,
		fixturePath:   strings.TrimSpace(os.Getenv("PERSONNEL_FIXTURE")),
		checkinPath:   checkinPath,
		devicesPath:   devicesPath,
		allowlistInit: strings.TrimSpace(os.Getenv("ALLOWLIST_INIT")),
		port:          port,
		timezone:      tz,
	}
}
