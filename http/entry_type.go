package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/innermond/dots"
)

func (s *Server) registerEntryTypeRoutes(router *mux.Router) {
	router.HandleFunc("", s.handleEntryTypeCreate).Methods("POST")
	router.HandleFunc("/{id}/edit", s.handleEntryTypeUpdate).Methods("PATCH")
}

func (s *Server) handleEntryTypeCreate(w http.ResponseWriter, r *http.Request) {
	var et dots.EntryType

	if err := json.NewDecoder(r.Body).Decode(&et); err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "new entry type: invalid json body"))
		return
	}

	err := s.EntryTypeService.CreateEntryType(r.Context(), &et)
	if err != nil {
		Error(w, r, err)
		return
	}

	w.Header().Set("Content-TYpe", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(et); err != nil {
		LogError(r, err)
		return
	}
}

func (s *Server) handleEntryTypeUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	var updata dots.EntryTypeUpdate
	if err := json.NewDecoder(r.Body).Decode(&updata); err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "edit entry type: invalid json body"))
		return
	}

	u := dots.UserFromContext(r.Context())
	updata.Tid = &u.ID

	if err := updata.Valid(); err != nil {
		Error(w, r, err)
		return
	}

	et, err := s.EntryTypeService.UpdateEntryType(r.Context(), id, &updata)
	if err != nil {
		Error(w, r, err)
		return
	}

	respondJSON[dots.EntryType](w, r, http.StatusOK, et)
}