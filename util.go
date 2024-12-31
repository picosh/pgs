package pgs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/picosh/pgs/storage"
	"github.com/picosh/send/utils"
	"github.com/picosh/utils"
	"github.com/picosh/utils/pipe"
	pipeLogger "github.com/picosh/utils/pipe/log"
)

type ApiConfig struct {
	Cfg     *ConfigSite
	Dbpool  db.DB
	Storage storage.StorageServe
}

func NewPicoPipeClient() *pipe.SSHClientInfo {
	return &pipe.SSHClientInfo{
		RemoteHost:     utils.GetEnv("PICO_PIPE_ENDPOINT", "pipe.pico.sh:22"),
		KeyLocation:    utils.GetEnv("PICO_PIPE_KEY", "ssh_data/term_info_ed25519"),
		KeyPassphrase:  utils.GetEnv("PICO_PIPE_PASSPHRASE", ""),
		RemoteHostname: utils.GetEnv("PICO_PIPE_REMOTE_HOST", "pipe.pico.sh"),
		RemoteUser:     utils.GetEnv("PICO_PIPE_USER", "pico"),
	}
}

func CreateLogger(space string) *slog.Logger {
	opts := &slog.HandlerOptions{
		AddSource: true,
	}
	log := slog.New(
		slog.NewTextHandler(os.Stdout, opts),
	)

	newLogger := log

	if strings.ToLower(utils.GetEnv("PICO_PIPE_ENABLED", "true")) == "true" {
		conn := NewPicoPipeClient()
		newLogger = pipeLogger.RegisterReconnectLogger(context.Background(), log, conn, 100, 10*time.Millisecond)
	}

	return newLogger.With("service", space)
}

func GetImgsBucketName(userID string) string {
	return userID
}

func GetAssetBucketName(userID string) string {
	return fmt.Sprintf("static-%s", userID)
}

func GetProjectName(entry *utils.FileEntry) string {
	if entry.Mode.IsDir() && strings.Count(entry.Filepath, string(os.PathSeparator)) == 0 {
		return entry.Filepath
	}

	dir := filepath.Dir(entry.Filepath)
	list := strings.Split(dir, string(os.PathSeparator))
	return list[1]
}

func GetAssetFileName(entry *utils.FileEntry) string {
	return entry.Filepath
}

type SubdomainProps struct {
	ProjectName string
	Username    string
}

func GetProjectFromSubdomain(subdomain string) (*SubdomainProps, error) {
	props := &SubdomainProps{}
	strs := strings.SplitN(subdomain, "-", 2)
	props.Username = strs[0]
	if len(strs) == 2 {
		props.ProjectName = strs[1]
	} else {
		props.ProjectName = props.Username
	}
	return props, nil
}

func RenderTemplate(cfg *ConfigSite, templates []string) (*template.Template, error) {
	files := make([]string, len(templates))
	copy(files, templates)
	files = append(
		files,
		"html/base.layout.tmpl",
	)

	ts, err := template.New("base").ParseFiles(files...)
	if err != nil {
		return nil, err
	}
	return ts, nil
}
