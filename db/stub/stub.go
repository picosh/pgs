package stub

import (
	"fmt"
	"log/slog"

	"github.com/picosh/pgs/db"
)

type StubDB struct {
	Logger *slog.Logger
}

var _ db.DB = (*StubDB)(nil)

func NewStubDB(logger *slog.Logger) *StubDB {
	d := &StubDB{
		Logger: logger,
	}
	d.Logger.Info("Connecting to test database")
	return d
}

var notImpl = fmt.Errorf("not implemented")

func (me *StubDB) FindUserByPubkey(username string, key string) (*db.User, error) {
	return nil, notImpl
}

func (me *StubDB) FindUser(userID string) (*db.User, error) {
	return nil, notImpl
}

func (me *StubDB) FindUserByName(name string) (*db.User, error) {
	return nil, notImpl
}

func (me *StubDB) FindFeature(userID string) *db.FeatureData {
	return nil
}

func (me *StubDB) Close() error {
	return notImpl
}

func (me *StubDB) FindTotalSizeForUser(userID string) (int, error) {
	return 0, notImpl
}

func (me *StubDB) InsertProject(userID, name, projectDir string) (string, error) {
	return "", notImpl
}

func (me *StubDB) UpdateProject(userID, name string) error {
	return notImpl
}

func (me *StubDB) LinkToProject(userID, projectID, projectDir string, commit bool) error {
	return notImpl
}

func (me *StubDB) RemoveProject(projectID string) error {
	return notImpl
}

func (me *StubDB) FindProjectByName(userID, name string) (*db.Project, error) {
	return &db.Project{}, notImpl
}

func (me *StubDB) FindProjectLinks(userID, name string) ([]*db.Project, error) {
	return []*db.Project{}, notImpl
}

func (me *StubDB) FindProjectsByPrefix(userID, prefix string) ([]*db.Project, error) {
	return []*db.Project{}, notImpl
}

func (me *StubDB) FindProjectsByUser(userID string) ([]*db.Project, error) {
	return []*db.Project{}, notImpl
}

func (me *StubDB) FindAllProjects(page *db.Pager, by string) (*db.Paginate[*db.Project], error) {
	return &db.Paginate[*db.Project]{}, notImpl
}
