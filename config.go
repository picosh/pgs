package pgs

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/picosh/utils"
)

type ConfigSite struct {
	Debug              bool
	SendgridKey        string
	Domain             string
	Port               string
	PortOverride       string
	Protocol           string
	DbURL              string
	StorageDir         string
	CacheTTL           time.Duration
	CacheControl       string
	MinioURL           string
	MinioUser          string
	MinioPass          string
	Space              string
	Issuer             string
	Secret             string
	SecretWebhook      string
	AllowedExt         []string
	HiddenPosts        []string
	MaxSize            uint64
	MaxAssetSize       int64
	MaxSpecialFileSize int64
	Logger             *slog.Logger
}

func (c *ConfigSite) AssetURL(username, projectName, fpath string) string {
	if username == projectName {
		return fmt.Sprintf(
			"%s://%s.%s/%s",
			c.Protocol,
			username,
			c.Domain,
			fpath,
		)
	}

	return fmt.Sprintf(
		"%s://%s-%s.%s/%s",
		c.Protocol,
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

func NewConfigSite() *ConfigSite {
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
	minioURL := utils.GetEnv("MINIO_URL", "")
	minioUser := utils.GetEnv("MINIO_ROOT_USER", "")
	minioPass := utils.GetEnv("MINIO_ROOT_PASSWORD", "")
	dbURL := utils.GetEnv("DATABASE_URL", "")

	cfg := ConfigSite{
		Domain:             domain,
		Port:               port,
		Protocol:           protocol,
		DbURL:              dbURL,
		StorageDir:         storageDir,
		CacheTTL:           cacheTTL,
		CacheControl:       cacheControl,
		MinioURL:           minioURL,
		MinioUser:          minioUser,
		MinioPass:          minioPass,
		Space:              "pgs",
		MaxSize:            maxSize,
		MaxAssetSize:       maxAssetSize,
		MaxSpecialFileSize: maxSpecialFileSize,
		Logger:             CreateLogger("pgs"),
	}

	return &cfg
}
