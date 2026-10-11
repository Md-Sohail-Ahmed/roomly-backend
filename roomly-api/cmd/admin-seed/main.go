package main

import (
	"context"
	"fmt"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"

	"roomly-api/internal/config"
	"roomly-api/internal/database"
	"roomly-api/internal/repository"
)

func main() {
	if err := run(); err != nil {
		log.Printf("admin seed failed: %v", err)
		os.Exit(1)
	}
	log.Print("Initial Roomly admin account is ready.")
}

func run() error {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("ROOMLY_BOOTSTRAP_ADMIN_EMAIL")))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return fmt.Errorf("ROOMLY_BOOTSTRAP_ADMIN_EMAIL must be the email of an existing active employee")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := database.Open(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := repository.NewUserRepository(db).PromoteFirstAdmin(ctx, email); err != nil {
		return err
	}
	return nil
}
