package dots

import (
	"context"

	"github.com/shopspring/decimal"
)

type Deed struct {
	ID *int `json:"id"`
	DeedUpdate
}

type DistributeDrain string

const (
	DistributeNewMany DistributeDrain = "new_many"
	DistributeNewFew  DistributeDrain = "new_few"
	DistributeOldMany DistributeDrain = "old_many"
	DistributeOldFew  DistributeDrain = "old_few"
)

func (d *Deed) Validate() error {
	return nil
}

type DeedService interface {
	CreateDeed(context.Context, *Deed) error
	UpdateDeed(context.Context, int, DeedUpdate) (*Deed, error)
	FindDeed(context.Context, DeedFilter) ([]*Deed, int, error)
	DeleteDeed(context.Context, int, DeedDelete) (int, error)
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

	DeletedAtFrom *PartialTime `json:"deleted_at_from,omitempty"`
	DeletedAtTo   *PartialTime `json:"deleted_at_to,omitempty"`
}

type DeedDelete struct {
	Undrain  bool `json:"undrain" presence_is:"true"`
	Resurect bool `json:"resurect" presence_is:"true"`
}

type DeedUpdate struct {
	CompanyID *int             `json:"company_id"`
	Title     *string          `json:"title"`
	Quantity  *float64         `json:"quantity"`
	Unit      *string          `json:"unit"`
	UnitPrice *decimal.Decimal `json:"unitprice"`

	Distribute map[int]float64 `json:"distribute,omitempty"`

	EntryTypeDistribute map[int]float64  `json:"entry_type_distribute,omitempty"`
	DistributeStrategy  *DistributeDrain `json:"distribute_strategy,omitempty"`
}

// TODO it panics violently!!!
/*func (du *DeedUpdate) UnmarshalJSON(b []byte) (err error) {
	d := DeedUpdate{}
	if err := json.Unmarshal(b, &d); err != nil {
		return err
	}
	*du = DeedUpdate(d)
	return
}*/

func (du *DeedUpdate) Valid() error {
	if du.Title == nil && du.Quantity == nil && du.Unit == nil {
		return Errorf(EINVALID, "at least title, quantity and unit are required")
	}

	return nil
}
