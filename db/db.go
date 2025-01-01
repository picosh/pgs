package db

import (
	"time"
)

type User struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CreatedAt *time.Time `json:"created_at"`
}

type PublicKey struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	Key       string     `json:"key"`
	CreatedAt *time.Time `json:"created_at"`
}

type FeatureData struct {
	// A permissions slice, used values: "write"
	Perms []string `json:"perms"`
	// Max total storage size allowed for a user
	StorageMax uint64 `json:"storage_max"`
	// Max file size allowed for a user
	FileMax int64 `json:"file_max"`
	// Max file size allowed for special files like `_headers` and `_redirects`
	SpecialFileMax int64 `json:"special_file_max"`
}

type Project struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	ProjectDir string     `json:"project_dir"`
	CreatedAt  *time.Time `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at"`
}

type DB interface {
	FindUser(userID string) (*User, error)
	FindUserByName(name string) (*User, error)
	FindUserByPubkey(username string, pubkey string) (*User, error)

	FindFeature(userID string) (*FeatureData, error)

	InsertProject(userID, name, projectDir string) (string, error)
	UpdateProject(userID, name string) error
	RemoveProject(projectID string) error
	LinkToProject(userID, projectID, projectDir string, commit bool) error
	FindProjectByName(userID, name string) (*Project, error)
	FindProjectLinks(userID, name string) ([]*Project, error)
	FindProjectsByUser(userID string) ([]*Project, error)
	FindProjectsByPrefix(userID, name string) ([]*Project, error)

	Close() error
}
