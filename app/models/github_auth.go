package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in github_auth.go

type GithubAuthRawData map[string]interface{}

func (d *GithubAuthRawData) Scan(val interface{}) error {
	switch v := val.(type) {
	case []byte:
		return json.Unmarshal(v, &d)
	case string:
		return json.Unmarshal([]byte(v), &d)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

func (d GithubAuthRawData) Value() (driver.Value, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("can't json marshal github auth data: %s", err)
	}

	return b, nil
}

func (d GithubAuthRawData) String() string {
	return fmt.Sprintf("<raw github data with %d fields>", len(d))
}

// gen:qs
type GithubAuth struct {
	gorm.Model

	AccessToken        string
	PrivateAccessToken string

	RawData GithubAuthRawData
	UserID  uint
	Login   string
}
