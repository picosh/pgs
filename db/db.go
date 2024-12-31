package db

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type Pager struct {
	Num  int
	Page int
}

type Project struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	ProjectDir string     `json:"project_dir"`
	Username   string     `json:"username"`
	Acl        ProjectAcl `json:"acl"`
	Blocked    string     `json:"blocked"`
	CreatedAt  *time.Time `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at"`
}

type ProjectAcl struct {
	Type string   `json:"type"`
	Data []string `json:"data"`
}

// Make the Attrs struct implement the driver.Valuer interface. This method
// simply returns the JSON-encoded representation of the struct.
func (p ProjectAcl) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Make the Attrs struct implement the sql.Scanner interface. This method
// simply decodes a JSON-encoded value into the struct fields.
func (p *ProjectAcl) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &p)
}

type DB interface{}
