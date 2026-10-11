package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"roomly-api/internal/config"
	"roomly-api/internal/database"
	"roomly-api/internal/handlers"
	"roomly-api/internal/middleware"
	"roomly-api/internal/repository"
	"roomly-api/internal/services"
)

func main() {
	if err := run(); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := database.Open(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	userRepo := repository.NewUserRepository(db)
	authService := services.NewAuthService(userRepo)
	authHandler := handlers.NewAuthHandler(authService, cfg.JWTSecret)
	roomRepo := repository.NewRoomRepository(db)
	roomHandler := handlers.NewRoomHandler(services.NewRoomService(roomRepo))
	amenityRepo := repository.NewAmenityRepository(db)
	amenityHandler := handlers.NewAmenityHandler(services.NewAmenityService(amenityRepo))
	bookingRepo := repository.NewBookingRepository(db)
	bookingHandler := handlers.NewBookingHandler(services.NewBookingService(bookingRepo))
	adminUserHandler := handlers.NewAdminUserHandler(services.NewAdminUserService(userRepo))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	withAuth := func(next http.Handler) http.Handler { return middleware.Auth(cfg.JWTSecret, userRepo, next) }
	adminOnly := func(next http.Handler) http.Handler { return withAuth(middleware.RequireRole("admin", next)) }
	mux.Handle("GET /api/v1/rooms", withAuth(http.HandlerFunc(roomHandler.List)))

	mux.Handle("GET /api/v1/rooms/{id}", withAuth(http.HandlerFunc(roomHandler.Get)))

	mux.Handle("POST /api/v1/rooms", adminOnly(http.HandlerFunc(roomHandler.Create)))

	mux.Handle("PUT /api/v1/rooms/{id}", adminOnly(http.HandlerFunc(roomHandler.Update)))

	mux.Handle("PATCH /api/v1/rooms/{id}/status", adminOnly(http.HandlerFunc(roomHandler.SetActive)))

	mux.Handle("GET /api/v1/amenities", withAuth(http.HandlerFunc(amenityHandler.List)))

	mux.Handle("GET /api/v1/amenities/{id}", withAuth(http.HandlerFunc(amenityHandler.Get)))

	mux.Handle("POST /api/v1/amenities", adminOnly(http.HandlerFunc(amenityHandler.Create)))

	mux.Handle("PUT /api/v1/amenities/{id}", adminOnly(http.HandlerFunc(amenityHandler.Update)))

	mux.Handle("DELETE /api/v1/amenities/{id}", adminOnly(http.HandlerFunc(amenityHandler.Delete)))

	mux.Handle("GET /api/v1/rooms/{id}/amenities", withAuth(http.HandlerFunc(amenityHandler.ListForRoom)))

	mux.Handle("POST /api/v1/rooms/{id}/amenities/{amenityID}", adminOnly(http.HandlerFunc(amenityHandler.AddToRoom)))

	mux.Handle("DELETE /api/v1/rooms/{id}/amenities/{amenityID}", adminOnly(http.HandlerFunc(amenityHandler.RemoveFromRoom)))

	mux.Handle("GET /api/v1/rooms/{id}/availability", withAuth(http.HandlerFunc(bookingHandler.Availability)))
	mux.Handle("GET /api/v1/rooms/{id}/schedule", withAuth(http.HandlerFunc(bookingHandler.RoomSchedule)))

	mux.Handle("POST /api/v1/bookings", withAuth(middleware.RequireRole("employee", http.HandlerFunc(bookingHandler.Create))))

	mux.Handle("GET /api/v1/bookings", adminOnly(http.HandlerFunc(bookingHandler.ListAll)))

	mux.Handle("GET /api/v1/bookings/mine", withAuth(middleware.RequireRole("employee", http.HandlerFunc(bookingHandler.ListMine))))

	mux.Handle("GET /api/v1/bookings/{id}", withAuth(http.HandlerFunc(bookingHandler.Get)))

	mux.Handle("POST /api/v1/bookings/{id}/cancel", withAuth(http.HandlerFunc(bookingHandler.Cancel)))

	mux.Handle("GET /api/v1/users", adminOnly(http.HandlerFunc(adminUserHandler.List)))

	mux.Handle("GET /api/v1/users/{id}", adminOnly(http.HandlerFunc(adminUserHandler.Get)))

	mux.Handle("PATCH /api/v1/users/{id}/status", adminOnly(http.HandlerFunc(adminUserHandler.SetActive)))

	mux.Handle("PATCH /api/v1/users/{id}/role", adminOnly(http.HandlerFunc(adminUserHandler.SetRole)))

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.AppPort),
		Handler:           requestIDMiddleware(middleware.RequestTimeout(8*time.Second, mux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Roomly server listening on port %d", cfg.AppPort)
		serverErrors <- server.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err

	case <-ctx.Done():
		log.Println("Shutting down Roomly server...")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return nil
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":  "ok",
		"service": "roomly",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("write health response: %v", err)
	}
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" || len(requestID) > 128 || strings.ContainsAny(requestID, "\r\n") {
			requestID = newRequestID()
		}

		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

func newRequestID() string {
	return time.Now().UTC().Format("20060102T150405.000000000Z")
}
