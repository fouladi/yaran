package yaran

import (
	"fmt"
	"strings"
	"time"
)

type Address struct {
	ID       int64  `json:"id,omitempty"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Birthday string `json:"birthday"`
	Address  string `json:"address"`
	Phone    string `json:"phone"`
	Mobile   string `json:"mobile"`
	Custom   string `json:"custom"`
	Notes    string `json:"notes"`
}

type Filters struct {
	Name     string
	Email    string
	Birthday string
	Address  string
	Phone    string
	Mobile   string
	Custom   string
	Notes    string
}

func (f Filters) HasFilters() bool {
	return f.Name != "" ||
		f.Email != "" ||
		f.Birthday != "" ||
		f.Address != "" ||
		f.Phone != "" ||
		f.Mobile != "" ||
		f.Custom != "" ||
		f.Notes != ""
}

func ValidateBirthday(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if _, err := time.Parse("2006-01-02", value); err != nil {
		return "", fmt.Errorf("birthday must use YYYY-MM-DD")
	}

	return value, nil
}

func NormalizeAddress(address Address) (Address, error) {
	address.Name = strings.TrimSpace(address.Name)
	address.Email = strings.TrimSpace(address.Email)
	address.Address = strings.TrimSpace(address.Address)
	address.Phone = strings.TrimSpace(address.Phone)
	address.Mobile = strings.TrimSpace(address.Mobile)
	address.Custom = strings.TrimSpace(address.Custom)
	address.Notes = strings.TrimSpace(address.Notes)

	birthday, err := ValidateBirthday(address.Birthday)
	if err != nil {
		return Address{}, err
	}
	address.Birthday = birthday

	return address, nil
}

func (f Filters) WithField(field string, value string) Filters {
	value = strings.TrimSpace(value)
	switch field {
	case "email":
		f.Email = value
	case "birthday":
		f.Birthday = value
	case "phone":
		f.Phone = value
	case "mobile":
		f.Mobile = value
	case "address":
		f.Address = value
	case "custom":
		f.Custom = value
	case "notes":
		f.Notes = value
	default:
		f.Name = value
	}
	return f
}
