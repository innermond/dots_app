package dots

import (
	"context"
)

type EntryType struct {
	ID          *int    `json:"id"`
	Code        *string `json:"code"`
	Description *string `json:"description"`
	Unit        *string `json:"unit"`
}

func (et *EntryType) Validate() error {
	if et.Code == nil || et.Unit == nil {
		return Errorf(EINVALID, "entry type code and unit must not be empty")
	}

	suspects := map[string]*string{
		"code":        et.Code,
		"description": et.Description,
		"unit":        et.Unit,
	}
	err := printable(suspects)
	if err != nil {
		return err
	}

	return nil
}

type EntryTypeService interface {
	CreateEntryType(context.Context, *EntryType) error
	UpdateEntryType(context.Context, int, EntryTypeUpdate) (*EntryType, error)
	FindEntryType(context.Context, EntryTypeFilterOrdered) ([]*EntryType, int, error)
	FindEntryTypeUnit(context.Context) ([]string, int, error)
	FindEntryTypeStats(context.Context, StatsFilter) (map[string]string, error)
	DeleteEntryType(context.Context, int, EntryTypeDelete) (int, error)
}

type EntryTypeFilter struct {
	ID          *int    `json:"id"`
	Code        *string `json:"code"`
	Description *string `json:"description"`
	Unit        *string `json:"unit"`

	Offset int `json:"offset"`
	Limit  int `json:"limit"`

	IsDeleted *bool `json:"is_deleted"`

	DeletedAtFrom *PartialTime `json:"deleted_at_from,omitempty"`
	DeletedAtTo   *PartialTime `json:"deleted_at_to,omitempty"`
}

type EntryTypeFilterOrdered struct {
	EntryTypeFilter

	ID          []string `json:"id"`
	Code        []string `json:"code"`
	Description []string `json:"description"`
	Unit        []string `json:"unit"`

	MaskID          string `json:"_mask_id"`
	MaskCode        string `json:"_mask_code"`
	MaskDescription string `json:"_mask_description"`
	MaskUnit        string `json:"_mask_unit"`
}

type StatsFilter struct {
	ID   *int    `json:"id"`
	Kind *string `json:"kind"`
}

type EntryTypeDelete struct {
	Hard     bool
	Resurect bool
}

type EntryTypeUpdate struct {
	Code        *string `json:"code"`
	Description *string `json:"description"`
	Unit        *string `json:"unit"`
}

func (etu *EntryTypeUpdate) Validate() error {
	if etu.Code == nil && etu.Unit == nil && etu.Description == nil {
		return Errorf(EINVALID, "entry type code or unit or description are required")
	}

	return nil
}
