package pgs

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/picosh/pgs/db"
	"github.com/picosh/pgs/storage"
	sst "github.com/picosh/pobj/storage"
	"github.com/picosh/utils"
)

type ConfigSite struct {
	CacheControl       string
	CacheTTL           time.Duration
	Domain             string
	MaxAssetSize       int64
	MaxSize            uint64
	MaxSpecialFileSize int64
	SshHost            string
	SshPort            string
	StorageDir         string
	TxtPrefix          string
	WebPort            string
	WebProtocol        string

	// This channel will receive the surrogate key for a project (e.g. static site)
	// which will inform the caching layer to clear the cache for that site.
	CacheClearingQueue chan string
	// Database layer; it's just an interface that could be implemented
	// with anything.
	DB     db.DB
	Logger *slog.Logger
	// Where we store the static assets uploaded to our service.
	Storage sst.ObjectStorage
}

func (c *ConfigSite) AssetURL(username, projectName, fpath string) string {
	if username == projectName {
		return fmt.Sprintf(
			"%s://%s.%s/%s",
			c.WebProtocol,
			username,
			c.Domain,
			fpath,
		)
	}

	return fmt.Sprintf(
		"%s://%s-%s.%s/%s",
		c.WebProtocol,
		username,
		projectName,
		c.Domain,
		fpath,
	)
}

var maxSize = uint64(25 * utils.MB)
var maxAssetSize = int64(10 * utils.MB)

// Needs to be small for caching files like _headers and _redirects.
var maxSpecialFileSize = int64(5 * utils.KB)

func NewConfigSite(logger *slog.Logger, dbpool db.DB, st storage.StorageServe) *ConfigSite {
	domain := utils.GetEnv("PGS_DOMAIN", "pgs.sh")
	port := utils.GetEnv("PGS_WEB_PORT", "3000")
	protocol := utils.GetEnv("PGS_PROTOCOL", "https")
	storageDir := utils.GetEnv("PGS_STORAGE_DIR", ".storage")
	cacheTTL, err := time.ParseDuration(utils.GetEnv("PGS_CACHE_TTL", ""))
	if err != nil {
		cacheTTL = 600 * time.Second
	}
	cacheControl := utils.GetEnv(
		"PGS_CACHE_CONTROL",
		fmt.Sprintf("max-age=%d", int(cacheTTL.Seconds())))

	sshHost := utils.GetEnv("PGS_SSH_HOST", "0.0.0.0")
	sshPort := utils.GetEnv("PGS_SSH_PORT", "2222")

	/*minioURL := utils.GetEnv("MINIO_URL", "")
	minioUser := utils.GetEnv("MINIO_ROOT_USER", "")
	minioPass := utils.GetEnv("MINIO_ROOT_PASSWORD", "")
	var st storage.StorageServe
	if minioURL == "" {
		st, err = storage.NewStorageFS(storageDir)
	} else {
		st, err = storage.NewStorageMinio(minioURL, minioUser, minioPass)
	}*/

	cfg := ConfigSite{
		CacheControl:       cacheControl,
		CacheTTL:           cacheTTL,
		Domain:             domain,
		MaxAssetSize:       maxAssetSize,
		MaxSize:            maxSize,
		MaxSpecialFileSize: maxSpecialFileSize,
		SshHost:            sshHost,
		SshPort:            sshPort,
		StorageDir:         storageDir,
		TxtPrefix:          "pgs",
		WebPort:            port,
		WebProtocol:        protocol,

		CacheClearingQueue: make(chan string, 100),
		DB:                 dbpool,
		Logger:             logger,
		Storage:            st,
	}

	return &cfg
}
