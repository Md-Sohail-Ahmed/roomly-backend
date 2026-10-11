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

type AmenityHandler struct{ amenities *services.AmenityService }

func NewAmenityHandler(amenities *services.AmenityService) *AmenityHandler {
	return &AmenityHandler{amenities: amenities}
}

func (h *AmenityHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit, err := parsePagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid pagination")
		return
	}
	items, total, err := h.amenities.List(r.Context(), limit, (page-1)*limit)
	if err != nil {
		log.Printf("list amenities: %v", err)
		writeError(w, http.StatusInternalServerError, "could not list amenities")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "total": total, "page": page, "limit": limit})
}

func (h *AmenityHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := amenityID(w, r)
	if !ok {
		return
	}
	item, err := h.amenities.Get(r.Context(), id)
	if err != nil {
		h.writeAmenityError(w, err, "could not get amenity")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

type amenityRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

func (h *AmenityHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req amenityRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeRequestDecodeError(w, err)
		return
	}
	item, err := h.amenities.Create(r.Context(), models.Amenity{Name: req.Name, Description: req.Description})
	if err != nil {
		h.writeAmenityError(w, err, "could not create amenity")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *AmenityHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := amenityID(w, r)
	if !ok {
		return
	}
	var req amenityRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeRequestDecodeError(w, err)
		return
	}
	item, err := h.amenities.Update(r.Context(), id, models.Amenity{Name: req.Name, Description: req.Description})
	if err != nil {
		h.writeAmenityError(w, err, "could not update amenity")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *AmenityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := amenityID(w, r)
	if !ok {
		return
	}
	if err := h.amenities.Delete(r.Context(), id); err != nil {
		h.writeAmenityError(w, err, "could not delete amenity")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AmenityHandler) ListForRoom(w http.ResponseWriter, r *http.Request) {
	roomID, ok := roomID(w, r)
	if !ok {
		return
	}
	items, err := h.amenities.ListForRoom(r.Context(), roomID)
	if err != nil {
		log.Printf("list amenities for room: %v", err)
		writeError(w, http.StatusInternalServerError, "could not list room amenities")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *AmenityHandler) AddToRoom(w http.ResponseWriter, r *http.Request) {
	roomID, ok := roomID(w, r)
	if !ok {
		return
	}
	amenityID, ok := amenityID(w, r)
	if !ok {
		return
	}
	if err := h.amenities.AddToRoom(r.Context(), roomID, amenityID); err != nil {
		h.writeAmenityError(w, err, "could not associate amenity")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AmenityHandler) RemoveFromRoom(w http.ResponseWriter, r *http.Request) {
	roomID, ok := roomID(w, r)
	if !ok {
		return
	}
	amenityID, ok := amenityID(w, r)
	if !ok {
		return
	}
	if err := h.amenities.RemoveFromRoom(r.Context(), roomID, amenityID); err != nil {
		h.writeAmenityError(w, err, "could not remove amenity association")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func amenityID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if r.PathValue("amenityID") != "" {
		id, err = strconv.ParseInt(r.PathValue("amenityID"), 10, 64)
	}
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid amenity ID")
		return 0, false
	}
	return id, true
}

func (h *AmenityHandler) writeAmenityError(w http.ResponseWriter, err error, message string) {
	switch {
	case errors.Is(err, services.ErrInvalidAmenity), errors.Is(err, services.ErrInvalidResourceID):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrAmenityNotFound), errors.Is(err, repository.ErrRoomNotFound), errors.Is(err, repository.ErrAssociationNotFound), errors.Is(err, repository.ErrRoomOrAmenityNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrAmenityExists), errors.Is(err, repository.ErrAssociationExists):
		writeError(w, http.StatusConflict, err.Error())
	default:
		log.Printf("%s: %v", message, err)
		writeError(w, http.StatusInternalServerError, message)
	}
}
