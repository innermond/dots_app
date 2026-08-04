package http

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/innermond/dots"
)

func (s *Server) registerDeedRoutes(router *mux.Router) {
	router.HandleFunc("", s.handleDeedCreate).Methods("POST")
	router.HandleFunc("/{id}", s.handleDeedPatch).Methods("PATCH")
	router.HandleFunc("", s.handleDeedFind).Methods("GET")
}

func (s *Server) handleDeedCreate(w http.ResponseWriter, r *http.Request) {
	var d dots.Deed

	if ok := inputJSON(w, r, &d, "create deed"); !ok {
		return
	}

	err := s.DeedService.CreateDeed(r.Context(), &d)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusCreated, &d)
}

func (s *Server) handleDeedPatch(w http.ResponseWriter, r *http.Request) {
	if _, found := r.URL.Query()["del"]; found {
		s.handleDeedDelete(w, r)
		return
	}

	s.handleDeedUpdate(w, r)
}

func (s *Server) handleDeedUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	var updata dots.DeedUpdate
	ok := inputJSON(w, r, &updata, "update deed")
	if !ok {
		return
	}

	d, err := s.DeedService.UpdateDeed(r.Context(), id, updata)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusOK, d)
}

func (s *Server) handleDeedFind(w http.ResponseWriter, r *http.Request) {
	filter := dots.DeedFilter{}
	input(w, r, &filter, "find deed")

	dd, n, err := s.DeedService.FindDeed(r.Context(), filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusFound, &foundResponse[[]*dots.Deed]{dd, affected{n}})
}

func (s *Server) handleDeedDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	filter := dots.DeedDelete{}
	input(w, r, &filter, "delete deed")

	n, err := s.DeedService.DeleteDeed(r.Context(), id, filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusFound, &affected{n})
}
