package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"roomly-api/internal/repository"
	"roomly-api/internal/services"
)

type AdminUserHandler struct{ users *services.AdminUserService }

func NewAdminUserHandler(users *services.AdminUserService) *AdminUserHandler {
	return &AdminUserHandler{users: users}
}

func (h *AdminUserHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pagination")
		return
	}
	users, total, err := h.users.List(r.Context(), limit, (page-1)*limit)
	if err != nil {
		h.writeError(w, err, "could not list users")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": users, "total": total, "page": page, "limit": limit})
}

func (h *AdminUserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := adminUserID(w, r)
	if !ok {
		return
	}
	user, err := h.users.Get(r.Context(), id)
	if err != nil {
		h.writeError(w, err, "could not get user")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *AdminUserHandler) SetActive(w http.ResponseWriter, r *http.Request) {
	id, ok := adminUserID(w, r)
	if !ok {
		return
	}
	var req struct {
		IsActive *bool `json:"is_active"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeRequestDecodeError(w, err)
		return
	}
	if req.IsActive == nil {
		writeError(w, http.StatusBadRequest, "is_active is required")
		return
	}
	user, err := h.users.SetActive(r.Context(), id, *req.IsActive)
	if err != nil {
		h.writeError(w, err, "could not update user status")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *AdminUserHandler) SetRole(w http.ResponseWriter, r *http.Request) {
	id, ok := adminUserID(w, r)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeRequestDecodeError(w, err)
		return
	}
	user, err := h.users.SetRole(r.Context(), id, req.Role)
	if err != nil {
		h.writeError(w, err, "could not update user role")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func adminUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return 0, false
	}
	return id, true
}

func (h *AdminUserHandler) writeError(w http.ResponseWriter, err error, message string) {
	switch {
	case errors.Is(err, services.ErrInvalidResourceID), errors.Is(err, services.ErrInvalidRole):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrUserNotFound):
		writeError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, repository.ErrLastActiveAdmin):
		writeError(w, http.StatusConflict, err.Error())
	default:
		log.Printf("%s: %v", message, err)
		writeError(w, http.StatusInternalServerError, message)
	}
}
