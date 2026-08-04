package dots

import (
	"context"

	"github.com/segmentio/ksuid"
)

type Drain struct {
	DeedID    int     `json:"deed_id"`
	EntryID   int     `json:"entry_id"`
	Quantity  float64 `json:"quantity"`
	IsDeleted bool    `json:"is_deleted"`
}

func (d *Drain) Validate() error {
	return nil
}

type DrainService interface {
	CreateOrUpdateDrain(context.Context, Drain) error
	//UpdateDrain(context.Context, int, *Drain) (*Drain, error)
	//FindDrain(context.Context, *DrainFilter) ([]*Drain, int, error)
}

type DrainFilter struct {
	DeedID   *int     `json:"deed_id"`
	EntryID  *int     `json:"entry_id"`
	Quantity *float64 `json:"quantity"`

	IsDeleted *bool `json:"is_deleted"`
	TID       *ksuid.KSUID

	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type DrainUpdate struct {
	DeedID   *int     `json:"deed_id"`
	EntryID  *int     `json:"entry_id"`
	Quantity *float64 `json:"quantity"`
}

func (d *DrainUpdate) Valid() error {
	if d.DeedID == nil && d.EntryID == nil && d.Quantity == nil {
		return Errorf(EINVALID, "all drain data is nil")
	}

	return nil
}
