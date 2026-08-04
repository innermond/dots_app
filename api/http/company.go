package http

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/innermond/dots"
)

func (s *Server) registerCompanyRoutes(router *mux.Router) {
	router.HandleFunc("", s.handleCompanyCreate).Methods("POST")
	router.HandleFunc("/{id}", s.handleCompanyPatch).Methods("PATCH")
	router.HandleFunc("", s.handleCompanyFind).Methods("GET")
	router.HandleFunc("/{id}", s.handleCompanyHardDelete).Methods("DELETE")
	router.HandleFunc("/stats", s.handleCompanyStats).Methods("GET")
	router.HandleFunc("/depletion", s.handleCompanyDepletion).Methods("GET")
}

func (s *Server) handleCompanyCreate(w http.ResponseWriter, r *http.Request) {
	var c dots.Company

	if ok := inputJSON(w, r, &c, "create company"); !ok {
		return
	}

	err := s.CompanyService.CreateCompany(r.Context(), &c)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusCreated, &c)
}

func (s *Server) handleCompanyPatch(w http.ResponseWriter, r *http.Request) {
	if _, found := r.URL.Query()["del"]; found {
		s.handleCompanyDelete(w, r)
		return
	}

	s.handleCompanyUpdate(w, r)
}

func (s *Server) handleCompanyUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	var updata dots.CompanyUpdate
	if ok := inputJSON(w, r, &updata, "update company"); !ok {
		return
	}

	c, err := s.CompanyService.UpdateCompany(r.Context(), id, updata)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusOK, c)
}

func (s *Server) handleCompanyFind(w http.ResponseWriter, r *http.Request) {
	filter := dots.CompanyFilter{}
	input(w, r, &filter, "find company")

	ee, n, err := s.CompanyService.FindCompany(r.Context(), filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusOK, &foundResponse[[]*dots.Company]{ee, affected{n}})
}

func (s *Server) handleCompanyDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	filter := dots.CompanyDelete{}
	// is body empty?
	if r.Body != http.NoBody {
		ok := inputJSON(w, r, &filter, "delete company")
		if !ok {
			return
		}
	}

	if _, found := r.URL.Query()["resurect"]; found {
		filter.Resurect = true
	}
	n, err := s.CompanyService.DeleteCompany(r.Context(), id, filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusFound, &affected{n})
}

func (s *Server) handleCompanyHardDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	var filter dots.CompanyDelete
	if r.Body != http.NoBody {
		ok := inputJSON(w, r, &filter, "hard delete company")
		if !ok {
			return
		}
	}
	filter.Hard = true

	n, err := s.CompanyService.DeleteCompany(r.Context(), id, filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusFound, &affected{n})
}

func (s *Server) handleCompanyStats(w http.ResponseWriter, r *http.Request) {
	filter := dots.CompanyFilter{}
	input(w, r, &filter, "stats company")

	ee, err := s.CompanyService.StatsCompany(r.Context(), filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	status := http.StatusOK

	outputJSON(w, r, status, &foundResponse[*dots.CompanyStats]{ee, affected{1}})
}

func (s *Server) handleCompanyDepletion(w http.ResponseWriter, r *http.Request) {
	filter := dots.CompanyFilter{}
	input(w, r, &filter, "depletion company")

	ee, n, err := s.CompanyService.DepletionCompany(r.Context(), filter)

	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusOK, &foundResponse[[]*dots.CompanyDepletion]{ee, affected{n}})
}
