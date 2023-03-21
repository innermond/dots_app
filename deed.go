package dots

import (
	"context"

	"github.com/shopspring/decimal"
)

type Deed struct {
	ID        int             `json:"id"`
	CompanyID int             `json:"company_id"`
	Title     string          `json:"title"`
	Quantity  float64         `json:"quantity"`
	Unit      string          `json:"unit"`
	UnitPrice decimal.Decimal `json:"unitprice"`
}

func (d *Deed) Validate() error {
	return nil
}

type DeedService interface {
	CreateDeed(context.Context, *Deed) error
	UpdateDeed(context.Context, int, *DeedUpdate) (*Deed, error)
	FindDeed(context.Context, *DeedFilter) ([]*Deed, int, error)
}

type DeedFilter struct {
	ID        *int             `json:"id"`
	CompanyID *int             `json:"company_id"`
	Title     *string          `json:"title"`
	Quantity  *float64         `json:"quantity"`
	Unit      *string          `json:"unit"`
	UnitPrice *decimal.Decimal `json:"unitprice"`

	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type DeedUpdate struct {
	CompanyID *int             `json:"company_id"`
	Title     *string          `json:"title"`
	Quantity  *float64         `json:"quantity"`
	Unit      *string          `json:"unit"`
	UnitPrice *decimal.Decimal `json:"unitprice"`
}

func (du *DeedUpdate) Valid() error {
	if du.Title == nil && du.Quantity == nil && du.Unit == nil {
		return Errorf(EINVALID, "at least title, quantity and unit are required")
	}

	return nil
}