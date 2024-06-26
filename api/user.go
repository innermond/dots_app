package dots

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/segmentio/ksuid"
)

type User struct {
	ID     ksuid.KSUID `json:"id"`
	Name   string      `json:"name"`
	Email  string      `json:"email"`
	ApiKey string      `json:"api_key"`
	Powers []Power     `json:"powers"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Auths []*Auth `json:"auths"`
}

type UserFilter struct {
	ID     *ksuid.KSUID `json:"id"`
	Email  *string      `json:"email"`
	ApiKey *string      `json:"api_key"`

	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type UserUpdate struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

func (u *User) ValidateCreate() error {
	// TODO regex for detecting white spaces
	if u.Name == "" {
		return Errorf(ECONFLICT, "User name required.")
	}
	return nil
}

var UserZero = &User{}

func UserIsZero(u *User) bool {
	return u.ID == ksuid.Nil &&
		u.Name == "" &&
		u.Email == "" &&
		u.ApiKey == "" &&
		u.Powers == nil &&
		u.CreatedAt.IsZero() &&
		u.UpdatedAt.IsZero()
}

type UserService interface {
	CreateUser(context.Context, *User) error
	FindUser(context.Context, UserFilter) ([]*User, int, error)
	FindUserByID(context.Context, ksuid.KSUID) (*User, error)
}

func (u User) Value() (driver.Value, error) {
	return json.Marshal(u)
}

func (u *User) Scan(v interface{}) error {
	b, ok := v.([]byte)
	if !ok {
		return errors.New("dots.user type assertion failed")
	}
	return json.Unmarshal(b, &u)
}

type userAlias User
type userTimeString struct {
	userAlias

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (u *User) UnmarshalJSON(b []byte) error {
	if string(b) == "null" || string(b) == `""` {
		return nil
	}

	var user userTimeString
	if err := json.Unmarshal(b, &user); err != nil {
		return err
	}
	layout := "2006-01-02T15:04:05+00:00"
	createdAt, err := time.Parse(layout, user.CreatedAt)
	if err != nil {
		return err
	}
	updatedAt, err := time.Parse(layout, user.UpdatedAt)
	if err != nil {
		return err
	}

	u.ID = user.ID
	u.Name = user.Name
	u.Email = user.Email
	u.ApiKey = user.ApiKey
	u.Powers = user.Powers
	u.CreatedAt = createdAt
	u.UpdatedAt = updatedAt

	return nil
}
