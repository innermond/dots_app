package dots

import (
	"context"
	"time"
)

type Entry struct {
	ID          int       `json:"id"`
	EntryTypeID int       `json:"entry_type_id"`
	DateAdded   time.Time `json:"date_added"`
	Quantity    float64   `json:"quantity"`
	CompanyID   int       `json:"company_id"`
}

func (e *Entry) Validate() error {
	return nil
}

type EntryService interface {
	CreateEntry(context.Context, *Entry) error
	UpdateEntry(context.Context, int, EntryUpdate) (*Entry, error)
	FindEntry(context.Context, EntryFilter) ([]*Entry, int, error)
	DeleteEntry(context.Context, EntryDelete) (int, error)
}

type EntryFilter struct {
	ID          *int       `json:"id"`
	EntryTypeID *int       `json:"entry_type_id"`
	DateAdded   *time.Time `json:"date_added"`
	Quantity    *float64   `json:"quantity"`
	CompanyID   *int       `json:"company_id"`

	Offset int `json:"offset"`
	Limit  int `json:"limit"`

	DeletedAtFrom *PartialTime `json:"deleted_at_from,omitempty"`
	DeletedAtTo   *PartialTime `json:"deleted_at_to,omitempty"`
}

type EntryDelete struct {
	EntryFilter

	Resurect bool
}

type EntryUpdate struct {
	EntryTypeID *int       `json:"entry_type_id"`
	DateAdded   *time.Time `json:"date_added"`
	Quantity    *float64   `json:"quantity"`
	CompanyID   *int       `json:"company_id"`
}

func (eu *EntryUpdate) Valid() error {
	if eu.EntryTypeID == nil || eu.Quantity == nil || eu.CompanyID == nil {
		return Errorf(EINVALID, "entry type, quantity and company are required")
	}

	return nil
}