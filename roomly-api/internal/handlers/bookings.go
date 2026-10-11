package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"roomly-api/internal/middleware"
	"roomly-api/internal/models"
	"roomly-api/internal/repository"
	"roomly-api/internal/services"
)

type BookingHandler struct{ bookings *services.BookingService }

func NewBookingHandler(bookings *services.BookingService) *BookingHandler {
	return &BookingHandler{bookings: bookings}
}

func (h *BookingHandler) Availability(w http.ResponseWriter, r *http.Request) {
	roomID, ok := roomID(w, r)
	if !ok {
		return
	}
	start, err := services.ParseTimestamp(r.URL.Query().Get("start_time"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "start_time must be RFC3339 with a timezone")
		return
	}
	end, err := services.ParseTimestamp(r.URL.Query().Get("end_time"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "end_time must be RFC3339 with a timezone")
		return
	}
	available, err := h.bookings.Availability(r.Context(), roomID, start, end)
	if err != nil {
		h.writeBookingError(w, err, "could not check availability")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"room_id": roomID, "start_time": start, "end_time": end, "available": available})
}

type bookingRequest struct {
	RoomID        int64  `json:"room_id"`
	Title         string `json:"title"`
	AttendeeCount int    `json:"attendee_count"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

func (h *BookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req bookingRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeRequestDecodeError(w, err)
		return
	}
	identity, ok := middleware.AuthenticatedUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	start, err := services.ParseTimestamp(req.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "start_time must be RFC3339 with a timezone")
		return
	}
	end, err := services.ParseTimestamp(req.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "end_time must be RFC3339 with a timezone")
		return
	}
	b, err := h.bookings.Create(r.Context(), models.Booking{RoomID: req.RoomID, UserID: identity.UserID, Title: req.Title, AttendeeCount: req.AttendeeCount, StartTime: start, EndTime: end})
	if err != nil {
		h.writeBookingError(w, err, "could not create booking")
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (h *BookingHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, nil)
}

func (h *BookingHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.AuthenticatedUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID := identity.UserID
	h.list(w, r, &userID)
}

func (h *BookingHandler) list(w http.ResponseWriter, r *http.Request, userID *int64) {
	page, limit, err := parsePagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pagination")
		return
	}
	items, total, err := h.bookings.List(r.Context(), userID, r.URL.Query().Get("status"), limit, (page-1)*limit)
	if err != nil {
		h.writeBookingError(w, err, "could not list bookings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "total": total, "page": page, "limit": limit})
}

func (h *BookingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := bookingID(w, r)
	if !ok {
		return
	}
	identity, ok := middleware.AuthenticatedUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	b, err := h.bookings.Get(r.Context(), id, identity.UserID, identity.Role == "admin")
	if err != nil {
		h.writeBookingError(w, err, "could not get booking")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *BookingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := bookingID(w, r)
	if !ok {
		return
	}
	identity, ok := middleware.AuthenticatedUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	b, err := h.bookings.Cancel(r.Context(), id, identity.UserID, identity.Role == "admin")
	if err != nil {
		h.writeBookingError(w, err, "could not cancel booking")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func bookingID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid booking ID")
		return 0, false
	}
	return id, true
}

func (h *BookingHandler) writeBookingError(w http.ResponseWriter, err error, message string) {
	switch {
	case errors.Is(err, services.ErrInvalidBooking), errors.Is(err, services.ErrInvalidResourceID), errors.Is(err, services.ErrBookingDuration), errors.Is(err, services.ErrBookingInPast):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrBookingForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, repository.ErrRoomNotFound), errors.Is(err, repository.ErrBookingNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, repository.ErrRoomInactive):
		writeError(w, http.StatusConflict, "room is inactive")
	case errors.Is(err, repository.ErrBookingCapacity), errors.Is(err, repository.ErrBookingConflict):
		writeError(w, http.StatusConflict, err.Error())
	default:
		log.Printf("%s: %v", message, err)
		writeError(w, http.StatusInternalServerError, message)
	}
}
