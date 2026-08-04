package http

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/innermond/dots"
)

func (s *Server) registerDrainRoutes(router *mux.Router) {
	router.HandleFunc("", s.handleDrainCreate).Methods("POST")
	//router.HandleFunc("/{id}/edit", s.handleDrainUpdate).Methods("PATCH")
	//router.HandleFunc("", s.handleDrainFind).Methods("GET")
}

func (s *Server) handleDrainCreate(w http.ResponseWriter, r *http.Request) {

	var et dots.Drain
	if err := json.NewDecoder(r.Body).Decode(&et); err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "new entry type: invalid json body"))
		return
	}

	err := s.DrainService.CreateOrUpdateDrain(r.Context(), et)
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

/*
func (s *Server) handleDrainUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	var updata dots.DrainUpdate
	if err := json.NewDecoder(r.Body).Decode(&updata); err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "edit entry type: invalid json body"))
		return
	}

	u := dots.UserFromContext(r.Context())
	updata.TID = &u.ID

	if err := updata.Valid(); err != nil {
		Error(w, r, err)
		return
	}

	et, err := s.DrainService.UpdateDrain(r.Context(), id, &updata)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON[dots.Drain](w, r, http.StatusOK, et)
}

func (s *Server) handleDrainFind(w http.ResponseWriter, r *http.Request) {
	var filter dots.DrainFilter
	ok := inputJSON[dots.DrainFilter](w, r, &filter)
	if !ok {
		return
	}

	ee, n, err := s.DrainService.FindDrain(r.Context(), &filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON[findDrainResponse](w, r, http.StatusFound, &findDrainResponse{Drains: ee, N: n})
}

type findDrainResponse struct {
	Drains []*dots.Drain `json:"entry_types"`
	N          int               `json:"n"`
}
*/
