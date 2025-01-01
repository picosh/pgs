package memory

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/picosh/pgs/db"
	"github.com/picosh/utils"
)

type MemoryDB struct {
	Logger   *slog.Logger
	Users    []*db.User
	Projects []*db.Project
	Pubkeys  []*db.PublicKey
	Feature  *db.FeatureData
}

var _ db.DB = (*MemoryDB)(nil)

func NewDBMemory(logger *slog.Logger) *MemoryDB {
	d := &MemoryDB{
		Logger: logger,
	}
	d.Logger.Info("Connecting to test database")
	return d
}

func (me *MemoryDB) SetupTestData() {
	user := &db.User{
		ID:   uuid.NewString(),
		Name: "testusr",
	}
	me.Users = append(me.Users, user)
	feature := &db.FeatureData{
		StorageMax:     uint64(25 * utils.MB),
		FileMax:        int64(10 * utils.MB),
		SpecialFileMax: int64(5 * utils.KB),
		Perms:          []string{"write"},
	}
	me.Feature = feature
}

var notImpl = fmt.Errorf("not implemented")

func (me *MemoryDB) FindUserByPubkey(username string, key string) (*db.User, error) {
	for _, pk := range me.Pubkeys {
		if pk.Key == key {
			return me.FindUser(pk.UserID)
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (me *MemoryDB) FindUser(userID string) (*db.User, error) {
	for _, user := range me.Users {
		if user.ID == userID {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (me *MemoryDB) FindUserByName(name string) (*db.User, error) {
	for _, user := range me.Users {
		if user.Name == name {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (me *MemoryDB) FindFeature(userID string) (*db.FeatureData, error) {
	return me.Feature, nil
}

func (me *MemoryDB) Close() error {
	return nil
}

func (me *MemoryDB) FindTotalSizeForUser(userID string) (int, error) {
	return 0, notImpl
}

func (me *MemoryDB) InsertProject(userID, name, projectDir string) (string, error) {
	id := uuid.NewString()
	me.Projects = append(me.Projects, &db.Project{
		ID:         id,
		UserID:     userID,
		Name:       name,
		ProjectDir: projectDir,
	})
	return id, nil
}

func (me *MemoryDB) UpdateProject(userID, name string) error {
	return notImpl
}

func (me *MemoryDB) LinkToProject(userID, projectID, projectDir string, commit bool) error {
	return notImpl
}

func (me *MemoryDB) RemoveProject(projectID string) error {
	return notImpl
}

func (me *MemoryDB) FindProjectByName(userID, name string) (*db.Project, error) {
	for _, project := range me.Projects {
		if project.UserID != userID {
			continue
		}

		if project.Name != name {
			continue
		}

		return project, nil
	}
	return &db.Project{}, fmt.Errorf("project not found")
}

func (me *MemoryDB) FindProjectLinks(userID, name string) ([]*db.Project, error) {
	return []*db.Project{}, notImpl
}

func (me *MemoryDB) FindProjectsByPrefix(userID, prefix string) ([]*db.Project, error) {
	return []*db.Project{}, notImpl
}

func (me *MemoryDB) FindProjectsByUser(userID string) ([]*db.Project, error) {
	return []*db.Project{}, notImpl
}
