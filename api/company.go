package dots

import (
	"context"
	"strings"
)

type Company struct {
	ID       int    `json:"id"`
	Longname string `json:"longname"`
	TIN      string `json:"tin"`
	RN       string `json:"rn"`
}

func (c *Company) Validate() error {
	if c.Longname == "" || c.TIN == "" || c.RN == "" {
		return Errorf(EINVALID, "all name, tax identification number and  registration number are required")
	}

	suspects := map[string]*string{
		"longname": &c.Longname,
		"tin":      &c.TIN,
		"rn":       &c.RN,
	}
	err := printable(suspects)
	if err != nil {
		return err
	}

	return nil
}

type CompanyFilter struct {
	ID       *int    `json:"id"`
	Longname *string `json:"longname"`
	TIN      *string `json:"tin"`
	RN       *string `json:"rn"`

	Offset int `json:"offset"`
	Limit  int `json:"limit"`

	IsDeleted *bool `json:"is_deleted"`

	DeletedAtFrom *PartialTime `json:"deleted_at_from,omitempty"`
	DeletedAtTo   *PartialTime `json:"deleted_at_to,omitempty"`
}

type CompanyDelete struct {
	// delete will be hard using "delete" sql kwyword
	Hard bool
	// update deletion field
	Resurect bool
}

type CompanyService interface {
	// CreateCompany creates a company and
	// hydrate the input param to reflect changes, hence
	// the input param must be a pointer
	CreateCompany(context.Context, *Company) error
	UpdateCompany(context.Context, int, CompanyUpdate) (*Company, error)
	FindCompany(context.Context, CompanyFilter) ([]*Company, int, error)
	DeleteCompany(context.Context, int, CompanyDelete) (int, error)
	StatsCompany(context.Context, CompanyFilter) (*CompanyStats, error)
	DepletionCompany(context.Context, CompanyFilter) ([]*CompanyDepletion, int, error)
}

type CompanyUpdate struct {
	Longname *string `json:"longname"`
	TIN      *string `json:"tin"`
	RN       *string `json:"rn"`
}

func (cu *CompanyUpdate) Validate() error {
	// required
	if cu.Longname == nil && cu.TIN == nil && cu.RN == nil {
		return Errorf(EINVALID, "al least one of name, tax identification number or registration number are required")
	}

	// trim white space
	if cu.Longname != nil {
		if len(*cu.Longname) == 0 {
			return Errorf(EINVALID, "name need content")
		}
		*cu.Longname = strings.Trim(*cu.Longname, " ")
	}
	if cu.TIN != nil {
		if len(*cu.TIN) == 0 {
			return Errorf(EINVALID, "tax identification  number need content")
		}
		*cu.TIN = strings.Trim(*cu.TIN, " ")
	}
	if cu.RN != nil {
		if len(*cu.RN) == 0 {
			return Errorf(EINVALID, "registration number need content")
		}
		*cu.RN = strings.Trim(*cu.RN, " ")
	}

	return nil
}

type CompanyStats struct {
	CountCompanies  int `json:"count_companies"`
	CountDeeds      int `json:"count_deeds"`
	CountEntries    int `json:"count_entries"`
	CountEntryTypes int `json:"count_entry_types"`
}

type CompanyDepletion struct {
	EntryTypeID     *int     `json:"entry_type_id"`
	Code            *string  `json:"code"`
	Description     *string  `json:"description,omitempty"`
	QuantityInitial *float64 `json:"quantity_initial"`
	QuantityDrained *float64 `json:"quantity_drained"`
}
