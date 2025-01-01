package db

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type Paginate[T any] struct {
	Data  []T
	Total int
}

type Pager struct {
	Num  int
	Page int
}

type User struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CreatedAt *time.Time `json:"created_at"`
}

type FeatureData struct {
	StorageMax     uint64 `json:"storage_max"`
	FileMax        int64  `json:"file_max"`
	SpecialFileMax int64  `json:"special_file_max"`
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

type DB interface {
	FindUser(userID string) (*User, error)
	FindUserByName(name string) (*User, error)
	FindUserByPubkey(username string, pubkey string) (*User, error)

	FindFeature(userID string) *FeatureData

	InsertProject(userID, name, projectDir string) (string, error)
	UpdateProject(userID, name string) error
	RemoveProject(projectID string) error
	LinkToProject(userID, projectID, projectDir string, commit bool) error
	FindProjectByName(userID, name string) (*Project, error)
	FindProjectLinks(userID, name string) ([]*Project, error)
	FindProjectsByUser(userID string) ([]*Project, error)
	FindProjectsByPrefix(userID, name string) ([]*Project, error)
	FindAllProjects(page *Pager, by string) (*Paginate[*Project], error)
}
