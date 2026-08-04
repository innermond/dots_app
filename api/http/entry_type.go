package http

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/innermond/dots"
)

func (s *Server) registerEntryTypeRoutes(router *mux.Router) {
	router.HandleFunc("", s.handleEntryTypeCreate).Methods("POST")
	router.HandleFunc("/{id}", s.handleEntryTypePatch).Methods("PATCH")
	router.HandleFunc("", s.handleEntryTypeUnitFind).Methods("GET").Queries("units", "{^$}")
	router.HandleFunc("", s.handleEntryTypeStats).Methods("GET").Queries("stats", "{^$}", "id", "{^$\\d+$}", "kind", "default")
	router.HandleFunc("", s.handleEntryTypeFind).Methods("GET")
	router.HandleFunc("/{id}", s.handleEntryTypeHardDelete).Methods("DELETE")
}

func (s *Server) handleEntryTypeCreate(w http.ResponseWriter, r *http.Request) {
	var et dots.EntryType

	if ok := inputJSON(w, r, &et, "create entry type"); !ok {
		return
	}

	err := s.EntryTypeService.CreateEntryType(r.Context(), &et)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusCreated, &et)
}

func (s *Server) handleEntryTypePatch(w http.ResponseWriter, r *http.Request) {

	tourist := dots.TouristFromContext(r.Context())
	tourist <- "handler"

	if _, found := r.URL.Query()["del"]; found {
		s.handleEntryTypeDelete(w, r)
		return
	}

	s.handleEntryTypeUpdate(w, r)
}

func (s *Server) handleEntryTypeUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	var updata dots.EntryTypeUpdate
	if ok := inputJSON(w, r, &updata, "edit entry type"); !ok {
		return
	}

	et, err := s.EntryTypeService.UpdateEntryType(r.Context(), id, updata)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusOK, et)
}

func (s *Server) handleEntryTypeFind(w http.ResponseWriter, r *http.Request) {
	// can accept missing r.Body
	//filter := dots.EntryTypeFilterOrdered{}
	//input(w, r, &filter, "find entry type")

	filterOrdered := dots.EntryTypeFilterOrdered{}
	keys := []string{"id", "code", "description", "unit", "limit", "offset", "_mask_id", "_mask_code", "_mask_description", "_mask_unit"}
	qp := r.URL.Query()

	for k, vv := range qp {
		is := false
		for _, existent := range keys {
			if k == existent {
				is = true
				break
			}
		}
		if !is {
			continue
		}

		switch k {
		case "id":
			filterOrdered.ID = vv
		case "code":
			filterOrdered.Code = vv
		case "description":
			filterOrdered.Description = vv
		case "unit":
			filterOrdered.Unit = vv
		case "limit":
			if v, err := strconv.Atoi(qp.Get(k)); err == nil {
				filterOrdered.Limit = v
			}
		case "offset":
			if v, err := strconv.Atoi(qp.Get(k)); err == nil {
				filterOrdered.Offset = v
			}
		case "_mask_id":
			filterOrdered.MaskID = qp.Get(k)
		case "_mask_code":
			filterOrdered.MaskCode = qp.Get(k)
		case "_mask_description":
			filterOrdered.MaskDescription = qp.Get(k)
		case "_mask_unit":
			filterOrdered.MaskUnit = qp.Get(k)
		}
	}

	ee, n, err := s.EntryTypeService.FindEntryType(r.Context(), filterOrdered)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusOK, &foundResponse[[]*dots.EntryType]{ee, affected{n}})
}

func (s *Server) handleEntryTypeStats(w http.ResponseWriter, r *http.Request) {
	// can accept missing r.Body
	filter := dots.StatsFilter{}
	input(w, r, &filter, "find entry type stats")

	ee, err := s.EntryTypeService.FindEntryTypeStats(r.Context(), filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusOK, &foundResponse[map[string]string]{ee, affected{len(ee)}})
}

func (s *Server) handleEntryTypeUnitFind(w http.ResponseWriter, r *http.Request) {
	// can accept missing r.Body
	filter := dots.EntryTypeFilter{}
	input(w, r, &filter, "find entry type unit")

	ee, n, err := s.EntryTypeService.FindEntryTypeUnit(r.Context())
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusOK, &foundResponse[[]string]{ee, affected{n}})
}

func (s *Server) handleEntryTypeDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	filter := dots.EntryTypeDelete{}
	if r.Body != http.NoBody {
		ok := inputJSON(w, r, &filter, "delete entry type")
		if !ok {
			return
		}
	}

	if _, found := r.URL.Query()["resurect"]; found {
		filter.Resurect = true
	}
	n, err := s.EntryTypeService.DeleteEntryType(r.Context(), id, filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusFound, &affected{n})
}

func (s *Server) handleEntryTypeHardDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		Error(w, r, dots.Errorf(dots.EINVALID, "invalid ID format"))
		return
	}

	var filter dots.EntryTypeDelete
	if r.Body != http.NoBody {
		ok := inputJSON(w, r, &filter, "hard delete entry type")
		if !ok {
			return
		}
	}
	filter.Hard = true

	n, err := s.EntryTypeService.DeleteEntryType(r.Context(), id, filter)
	if err != nil {
		Error(w, r, err)
		return
	}

	outputJSON(w, r, http.StatusOK, &affected{n})
}
