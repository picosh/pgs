package pgs

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/ssh"
	"github.com/picosh/pgs/db"
	sendUtils "github.com/picosh/send/utils"
	"github.com/picosh/utils"
	"github.com/picosh/utils/pipe"
)

func NewPicoPipeClient() *pipe.SSHClientInfo {
	return &pipe.SSHClientInfo{
		RemoteHost:     utils.GetEnv("PICO_PIPE_ENDPOINT", "pipe.pico.sh:22"),
		KeyLocation:    utils.GetEnv("PICO_PIPE_KEY", "ssh_data/term_info_ed25519"),
		KeyPassphrase:  utils.GetEnv("PICO_PIPE_PASSPHRASE", ""),
		RemoteHostname: utils.GetEnv("PICO_PIPE_REMOTE_HOST", "pipe.pico.sh"),
		RemoteUser:     utils.GetEnv("PICO_PIPE_USER", "pico"),
	}
}

func GetImgsBucketName(userID string) string {
	return userID
}

func GetAssetBucketName(userID string) string {
	return fmt.Sprintf("static-%s", userID)
}

func GetAssetFileName(entry *sendUtils.FileEntry) string {
	return entry.Filepath
}

func GetProjectName(entry *sendUtils.FileEntry) string {
	if entry.Mode.IsDir() && strings.Count(entry.Filepath, string(os.PathSeparator)) == 0 {
		return entry.Filepath
	}

	dir := filepath.Dir(entry.Filepath)
	list := strings.Split(dir, string(os.PathSeparator))
	return list[1]
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

func LoggerWithUser(logger *slog.Logger, user *db.User) *slog.Logger {
	return logger.With("user", user.Name, "userId", user.ID)
}

// The ssh app uses `user_id` to determine the current user
// for the running session.  This value must be set before our
// upload handler is called.
func SetUserIdForSession(ctx ssh.Context, userID string) {
	if ctx.Permissions().Extensions == nil {
		ctx.Permissions().Extensions = map[string]string{}
	}
	ctx.Permissions().Extensions["user_id"] = userID
}

// This fn grabs the user_id from the session's extensions.
func GetUserIDFromSession(sesh ssh.Session) (string, error) {
	if sesh.Permissions().Extensions == nil {
		return "", fmt.Errorf("no extensions map created for ssh session")
	}
	userID := sesh.Permissions().Extensions["user_id"]
	if userID == "" {
		return "", fmt.Errorf("extension `user_id` not set for ssh session")
	}
	return userID, nil
}
