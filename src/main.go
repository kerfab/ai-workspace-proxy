// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	if err := ValidatePolicyCatalog(); err != nil {
		log.Fatalf("policy catalog error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o700); err != nil {
		log.Fatalf("db dir error: %v", err)
	}

	db, err := OpenSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("db open error: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(); err != nil {
		log.Fatalf("db init error: %v", err)
	}
	startLogRetentionCleanup(store)

	crypto := NewCrypto(cfg.EncryptionKey)
	app := NewApp(cfg, store, crypto)

	server := &http.Server{
		Addr:    cfg.BindAddr,
		Handler: app.Routes(),
	}

	log.Printf("%s listening on %s", cfg.AppName, cfg.BindAddr)
	log.Printf("proxy relays available under %s/gmail.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/people.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/calendar.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/drive.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/docs.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/sheets.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/slides.googleapis.com/...", cfg.BaseURL)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func startLogRetentionCleanup(store *Store) {
	if err := store.CleanupExpiredLogs(); err != nil {
		log.Printf("log retention cleanup error: %v", err)
	}
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := store.CleanupExpiredLogs(); err != nil {
				log.Printf("log retention cleanup error: %v", err)
			}
		}
	}()
}
