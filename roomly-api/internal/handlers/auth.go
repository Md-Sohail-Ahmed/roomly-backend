package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"roomly-api/internal/repository"
	"roomly-api/internal/services"
)

type AuthHandler struct {
	auth      *services.AuthService
	jwtSecret string
}

func NewAuthHandler(auth *services.AuthService, jwtSecret string) *AuthHandler {
	return &AuthHandler{auth: auth, jwtSecret: jwtSecret}
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		if errors.Is(err, errJSONContentType) {
			writeError(w, http.StatusUnsupportedMediaType, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	user, err := h.auth.Register(
		r.Context(),
		req.Name,
		req.Email,
		req.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidName),
			errors.Is(err, services.ErrInvalidEmail),
			errors.Is(err, services.ErrInvalidPassword):
			writeError(w, http.StatusBadRequest, err.Error())

		case errors.Is(err, repository.ErrEmailAlreadyExists):
			writeError(w, http.StatusConflict, "email is already registered")

		default:
			log.Printf("register user failed: %v", err)
			writeError(w, http.StatusInternalServerError, "could not register user")
		}
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		if errors.Is(err, errJSONContentType) {
			writeError(w, http.StatusUnsupportedMediaType, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	user, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		log.Printf("login failed: %v", err)
		writeError(w, http.StatusInternalServerError, "could not log in")
		return
	}

	token, err := services.GenerateToken(user, h.jwtSecret)
	if err != nil {
		log.Printf("generate login token: %v", err)
		writeError(w, http.StatusInternalServerError, "could not log in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("encode JSON response: %v", err)
	}
}
