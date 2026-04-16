package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o700); err != nil {
		log.Fatalf("db dir error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DeniedLogPath), 0o700); err != nil {
		log.Fatalf("logs dir error: %v", err)
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

	crypto := NewCrypto(cfg.EncryptionKey)
	policy, err := LoadPolicy(cfg.PolicyPath)
	if err != nil {
		log.Fatalf("policy load error: %v", err)
	}
	deniedLogger := NewDeniedLogger(cfg.DeniedLogPath, 10*1024*1024, 5)

	app := NewApp(cfg, store, crypto, policy, deniedLogger)

	server := &http.Server{
		Addr:    cfg.BindAddr,
		Handler: app.Routes(),
	}

	log.Printf("%s listening on %s", cfg.AppName, cfg.BindAddr)
	log.Printf("policy loaded from %s", cfg.PolicyPath)
	log.Printf("proxy relays available under %s/gmail.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/calendar.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/drive.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/docs.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/sheets.googleapis.com/...", cfg.BaseURL)
	log.Printf("proxy relays available under %s/slides.googleapis.com/...", cfg.BaseURL)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
