package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"roomly-api/internal/models"
	"roomly-api/internal/repository"
	"roomly-api/internal/services"
)

type RoomHandler struct{ rooms *services.RoomService }

func NewRoomHandler(rooms *services.RoomService) *RoomHandler { return &RoomHandler{rooms: rooms} }

func (h *RoomHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pagination")
		return
	}
	minCapacity := 0
	if value := r.URL.Query().Get("min_capacity"); value != "" {
		minCapacity, err = strconv.Atoi(value)
		if err != nil || minCapacity < 0 {
			writeError(w, http.StatusBadRequest, "min_capacity must be zero or greater")
			return
		}
	}
	rooms, total, err := h.rooms.List(r.Context(), models.RoomFilter{Location: r.URL.Query().Get("location"), MinCapacity: minCapacity, Limit: limit, Offset: (page - 1) * limit})
	if err != nil {
		log.Printf("list rooms: %v", err)
		writeError(w, http.StatusInternalServerError, "could not list rooms")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": rooms, "total": total, "page": page, "limit": limit})
}

func (h *RoomHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := roomID(w, r)
	if !ok {
		return
	}
	room, err := h.rooms.Get(r.Context(), id)
	if err != nil {
		h.writeRoomError(w, err, "could not get room")
		return
	}
	writeJSON(w, http.StatusOK, room)
}

type roomRequest struct {
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
	Location string `json:"location"`
}

func (h *RoomHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req roomRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeDecodeError(w, err)
		return
	}
	room, err := h.rooms.Create(r.Context(), models.Room{Name: req.Name, Capacity: req.Capacity, Location: req.Location})
	if err != nil {
		h.writeRoomError(w, err, "could not create room")
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

func (h *RoomHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := roomID(w, r)
	if !ok {
		return
	}
	var req roomRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeDecodeError(w, err)
		return
	}
	room, err := h.rooms.Update(r.Context(), id, models.Room{Name: req.Name, Capacity: req.Capacity, Location: req.Location})
	if err != nil {
		h.writeRoomError(w, err, "could not update room")
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func (h *RoomHandler) SetActive(w http.ResponseWriter, r *http.Request) {
	id, ok := roomID(w, r)
	if !ok {
		return
	}
	var req struct {
		IsActive *bool `json:"is_active"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeDecodeError(w, err)
		return
	}
	if req.IsActive == nil {
		writeError(w, http.StatusBadRequest, "is_active is required")
		return
	}
	room, err := h.rooms.SetActive(r.Context(), id, *req.IsActive)
	if err != nil {
		h.writeRoomError(w, err, "could not update room status")
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func roomID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid room ID")
		return 0, false
	}
	return id, true
}

func parsePagination(r *http.Request) (int, int, error) {
	page, limit := 1, 20
	var err error
	if raw := r.URL.Query().Get("page"); raw != "" {
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 {
			return 0, 0, errors.New("invalid page")
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, errors.New("invalid limit")
		}
	}
	if page-1 > int(^uint(0)>>1)/limit {
		return 0, 0, errors.New("pagination overflow")
	}
	return page, limit, nil
}

func (h *RoomHandler) writeDecodeError(w http.ResponseWriter, err error) {
	if errors.Is(err, errJSONContentType) {
		writeError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, "invalid request body")
}

func (h *RoomHandler) writeRoomError(w http.ResponseWriter, err error, message string) {
	switch {
	case errors.Is(err, services.ErrInvalidRoom), errors.Is(err, services.ErrInvalidResourceID):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrRoomNotFound):
		writeError(w, http.StatusNotFound, "room not found")
	default:
		log.Printf("%s: %v", message, err)
		writeError(w, http.StatusInternalServerError, message)
	}
}
