package db

import (
	"time"
)

type User interface {
	GetID() string
	GetName() string
}

type PublicKey interface {
	GetUserID() string
	GetPubkey() string
}

type Feature interface {
	// A permissions slice, used values: "write"
	GetPerms() []string
	// Max total storage size allowed for a user
	GetStorageMax() uint64
	// Max file size allowed for a user
	GetFileMax() int64
	// Max file size allowed for special files like `_headers` and `_redirects`
	GetSpecialFileMax() int64
}

type FeatureData struct {
	perms          []string
	storageMax     uint64
	fileMax        int64
	specialFileMax int64
}

func NewFeatureData(perms []string, storageMax uint64, fileMax, specialFileMax int64) *FeatureData {
	return &FeatureData{
		perms:          perms,
		storageMax:     storageMax,
		fileMax:        fileMax,
		specialFileMax: specialFileMax,
	}
}

func (m *FeatureData) GetPerms() []string {
	return m.perms
}

func (m *FeatureData) GetStorageMax() uint64 {
	return m.storageMax
}

func (m *FeatureData) GetFileMax() int64 {
	return m.fileMax
}

func (m *FeatureData) GetSpecialFileMax() int64 {
	return m.specialFileMax
}

type Project interface {
	GetID() string
	GetUserID() string
	GetName() string
	GetProjectDir() string
	GetUpdatedAt() *time.Time
}

type DB interface {
	FindUser(userID string) (User, error)
	FindUserByName(name string) (User, error)
	FindUserByPubkey(username string, pubkey string) (User, error)

	FindFeature(userID string) (Feature, error)

	InsertProject(userID, name, projectDir string) (string, error)
	UpdateProject(userID, name string) error
	RemoveProject(projectID string) error
	LinkToProject(userID, projectID, projectDir string, commit bool) error
	FindProjectByName(userID, name string) (Project, error)
	FindProjectLinks(userID, name string) ([]Project, error)
	FindProjectsByUser(userID string) ([]Project, error)
	FindProjectsByPrefix(userID, name string) ([]Project, error)

	Close() error
}
