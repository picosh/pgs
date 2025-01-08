package memory

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/picosh/pgs/db"
	"github.com/picosh/utils"
)

type MemUser struct {
	id   string
	name string
}

func (m *MemUser) GetID() string {
	return m.id
}

func (m *MemUser) GetName() string {
	return m.name
}

type MemProject struct {
	id         string
	userID     string
	name       string
	projectDir string
	updatedAt  *time.Time
}

func (m *MemProject) GetID() string {
	return m.id
}

func (m *MemProject) GetUserID() string {
	return m.userID
}

func (m *MemProject) GetName() string {
	return m.name
}

func (m *MemProject) GetProjectDir() string {
	return m.projectDir
}

func (m *MemProject) GetUpdatedAt() *time.Time {
	return m.updatedAt
}

type MemPublicKey struct {
	userID string
	pubkey string
}

func NewMemPublicKey(userID, pubkey string) *MemPublicKey {
	return &MemPublicKey{
		userID: userID,
		pubkey: pubkey,
	}
}

func (m *MemPublicKey) GetUserID() string {
	return m.userID
}

func (m *MemPublicKey) GetPubkey() string {
	return m.pubkey
}

type MemoryDB struct {
	Logger   *slog.Logger
	Users    []*MemUser
	Projects []*MemProject
	Pubkeys  []*MemPublicKey
	Feature  *db.FeatureData
}

var _ db.DB = (*MemoryDB)(nil)

func NewDBMemory(logger *slog.Logger) *MemoryDB {
	d := &MemoryDB{
		Logger: logger,
	}
	d.Logger.Info("Connecting to our in-memory database. All data created during runtime will be lost on exit.")
	return d
}

func (me *MemoryDB) SetupTestData() {
	user := &MemUser{
		id:   uuid.NewString(),
		name: "testusr",
	}
	me.Users = append(me.Users, user)
	feature := db.NewFeatureData(
		[]string{"write"},
		uint64(25*utils.MB),
		int64(10*utils.MB),
		int64(5*utils.KB),
	)
	me.Feature = feature
}

var notImpl = fmt.Errorf("not implemented")

func (me *MemoryDB) FindUserByPubkey(username string, key string) (db.User, error) {
	for _, pk := range me.Pubkeys {
		if pk.GetPubkey() == key {
			return me.FindUser(pk.GetUserID())
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (me *MemoryDB) FindUser(userID string) (db.User, error) {
	for _, user := range me.Users {
		if user.GetID() == userID {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (me *MemoryDB) FindUserByName(name string) (db.User, error) {
	for _, user := range me.Users {
		if user.GetName() == name {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (me *MemoryDB) FindFeature(userID string) (db.Feature, error) {
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
	me.Projects = append(me.Projects, &MemProject{
		id:         id,
		userID:     userID,
		name:       name,
		projectDir: projectDir,
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

func (me *MemoryDB) FindProjectByName(userID, name string) (db.Project, error) {
	for _, project := range me.Projects {
		if project.GetUserID() != userID {
			continue
		}

		if project.GetName() != name {
			continue
		}

		return project, nil
	}
	return nil, fmt.Errorf("project not found by name %s", name)
}

func (me *MemoryDB) FindProjectLinks(userID, name string) ([]db.Project, error) {
	return []db.Project{}, notImpl
}

func (me *MemoryDB) FindProjectsByPrefix(userID, prefix string) ([]db.Project, error) {
	return []db.Project{}, notImpl
}

func (me *MemoryDB) FindProjectsByUser(userID string) ([]db.Project, error) {
	return []db.Project{}, notImpl
}
