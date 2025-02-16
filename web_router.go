package pgs

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	_ "net/http/pprof"

	"github.com/darkweak/souin/pkg/middleware"
	"github.com/darkweak/storages/core"
	"github.com/picosh/pgs/db"
	"github.com/picosh/pgs/storage"
	sst "github.com/picosh/pobj/storage"
	"google.golang.org/protobuf/proto"
)

type CtxSubdomainKey struct{}

func GetSubdomain(r *http.Request) string {
	return r.Context().Value(CtxSubdomainKey{}).(string)
}

func GetCustomDomain(host string, prefix string) string {
	txt := fmt.Sprintf("_%s.%s", prefix, host)
	records, err := net.LookupTXT(txt)
	if err != nil {
		return ""
	}

	for _, v := range records {
		return strings.TrimSpace(v)
	}

	return ""
}

type WebRouter struct {
	DB         db.DB
	Logger     *slog.Logger
	Storage    storage.StorageServe
	Domain     string
	TxtPrefix  string
	RootRouter *http.ServeMux
	UserRouter *http.ServeMux
}

func NewWebRouter(logger *slog.Logger, dbpool db.DB, storage storage.StorageServe, domain, txtPrefix string) *WebRouter {
	router := &WebRouter{
		Logger:    logger,
		DB:        dbpool,
		Storage:   storage,
		Domain:    domain,
		TxtPrefix: txtPrefix,
	}
	router.InitRouters()
	return router
}

func (web *WebRouter) InitRouters() {
	// ensure legacy router is disabled
	// GODEBUG=httpmuxgo121=0

	// root domain
	rootRouter := http.NewServeMux()
	rootRouter.HandleFunc("GET /check", web.checkHandler)
	rootRouter.Handle("GET /robots.txt", web.serveFile("robots.txt", "text/plain"))
	rootRouter.Handle("GET /", web.serveFile("index.html", "text/html"))
	web.RootRouter = rootRouter

	// subdomain or custom domains
	userRouter := http.NewServeMux()
	userRouter.HandleFunc("GET /{fname...}", web.AssetRequest)
	userRouter.HandleFunc("GET /{$}", web.AssetRequest)
	web.UserRouter = userRouter
}

func (web *WebRouter) serveFile(file string, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := web.Logger

		contents, err := os.ReadFile(fmt.Sprintf("public/%s", file))
		if err != nil {
			logger.Error(
				"could not read file",
				"fname", file,
				"err", err.Error(),
			)
			http.Error(w, "file not found", 404)
		}

		w.Header().Add("Content-Type", contentType)

		_, err = w.Write(contents)
		if err != nil {
			logger.Error(
				"could not write http response",
				"file", file,
				"err", err.Error(),
			)
		}
	}
}

func (web *WebRouter) checkHandler(w http.ResponseWriter, r *http.Request) {
	dbpool := web.DB
	logger := web.Logger

	hostDomain := r.URL.Query().Get("domain")
	appDomain := strings.Split(web.Domain, ":")[0]

	if !strings.Contains(hostDomain, appDomain) {
		subdomain := GetCustomDomain(hostDomain, web.TxtPrefix)
		props, err := GetProjectFromSubdomain(subdomain)
		if err != nil {
			logger.Error(
				"could not get project from subdomain",
				"subdomain", subdomain,
				"err", err.Error(),
			)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		u, err := dbpool.FindUserByName(props.Username)
		if err != nil {
			logger.Error("could not find user", "err", err.Error())
			w.WriteHeader(http.StatusNotFound)
			return
		}

		logger = logger.With(
			"user", u.GetName(),
			"project", props.ProjectName,
		)
		p, err := dbpool.FindProjectByName(u.GetID(), props.ProjectName)
		if err != nil {
			logger.Error(
				"could not find project for user",
				"user", u.GetName(),
				"project", props.ProjectName,
				"err", err.Error(),
			)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if u != nil && p != nil {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func (web *WebRouter) CacheMgmt(ctx context.Context, httpCache *middleware.SouinBaseHandler) {
	storer := httpCache.Storers[0]
	drain := CreateSubCacheDrain(ctx, web.Logger)

	for {
		scanner := bufio.NewScanner(drain)
		for scanner.Scan() {
			surrogateKey := strings.TrimSpace(scanner.Text())
			web.Logger.Info("received cache-drain item", "surrogateKey", surrogateKey)

			if surrogateKey == "*" {
				storer.DeleteMany(".+")
				err := httpCache.SurrogateKeyStorer.Destruct()
				if err != nil {
					web.Logger.Error("could not clear cache and surrogate key store", "err", err)
				} else {
					web.Logger.Info("successfully cleared cache and surrogate keys store")
				}
				continue
			}

			var header http.Header = map[string][]string{}
			header.Add("Surrogate-Key", surrogateKey)

			ck, _ := httpCache.SurrogateKeyStorer.Purge(header)
			for _, key := range ck {
				key, _ = strings.CutPrefix(key, core.MappingKeyPrefix)
				if b := storer.Get(core.MappingKeyPrefix + key); len(b) > 0 {
					var mapping core.StorageMapper
					if e := proto.Unmarshal(b, &mapping); e == nil {
						for k := range mapping.GetMapping() {
							qkey, _ := url.QueryUnescape(k)
							web.Logger.Info(
								"deleting key from surrogate cache",
								"surrogateKey", surrogateKey,
								"key", qkey,
							)
							storer.Delete(qkey)
						}
					}
				}

				qkey, _ := url.QueryUnescape(key)
				web.Logger.Info(
					"deleting from cache",
					"surrogateKey", surrogateKey,
					"key", core.MappingKeyPrefix+qkey,
				)
				storer.Delete(core.MappingKeyPrefix + qkey)
			}
		}
	}
}

var imgRegex = regexp.MustCompile("(.+.(?:jpg|jpeg|png|gif|webp|svg))(/.+)")

func (web *WebRouter) AssetRequest(w http.ResponseWriter, r *http.Request) {
	fname := r.PathValue("fname")
	if imgRegex.MatchString(fname) {
		web.ImageRequest(w, r)
		return
	}
	web.ServeAsset(fname, nil, false, w, r)
}

func (web *WebRouter) ImageRequest(w http.ResponseWriter, r *http.Request) {
	rawname := r.PathValue("fname")
	matches := imgRegex.FindStringSubmatch(rawname)
	fname := rawname
	imgOpts := ""
	if len(matches) >= 2 {
		fname = matches[1]
	}
	if len(matches) >= 3 {
		imgOpts = matches[2]
	}

	opts, err := storage.UriToImgProcessOpts(imgOpts)
	if err != nil {
		errMsg := fmt.Sprintf("error processing img options: %s", err.Error())
		web.Logger.Error("error processing img options", "err", errMsg)
		http.Error(w, errMsg, http.StatusUnprocessableEntity)
		return
	}

	web.ServeAsset(fname, opts, false, w, r)
}

func (web *WebRouter) ServeAsset(fname string, opts *storage.ImgProcessOpts, fromImgs bool, w http.ResponseWriter, r *http.Request) {
	subdomain := GetSubdomain(r)

	logger := web.Logger.With(
		"subdomain", subdomain,
		"filename", fname,
		"url", fmt.Sprintf("%s%s", r.Host, r.URL.Path),
		"host", r.Host,
	)

	props, err := GetProjectFromSubdomain(subdomain)
	if err != nil {
		logger.Info(
			"could not determine project from subdomain",
			"err", err,
		)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	logger = logger.With(
		"project", props.ProjectName,
		"user", props.Username,
	)

	user, err := web.DB.FindUserByName(props.Username)
	if err != nil {
		logger.Info("user not found")
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	logger = logger.With(
		"userId", user.GetID(),
	)

	// TODO: this could probably be cleaned up more
	// imgs wont have a project directory
	projectDir := ""
	var bucket sst.Bucket
	// imgs has a different bucket directory
	if fromImgs {
		bucket, err = web.Storage.GetBucket(GetImgsBucketName(user.GetID()))
	} else {
		bucket, err = web.Storage.GetBucket(GetAssetBucketName(user.GetID()))
		project, err := web.DB.FindProjectByName(user.GetID(), props.ProjectName)
		if err != nil {
			logger.Info("project not found", "project", props.ProjectName)
			http.Error(w, fmt.Sprintf("project not found %s", props.ProjectName), http.StatusNotFound)
			return
		}

		logger = logger.With(
			"projectId", project.GetID(),
			"project", project.GetName(),
		)

		projectDir = project.GetProjectDir()
	}

	if err != nil {
		logger.Info("bucket not found")
		http.Error(w, "bucket not found", http.StatusNotFound)
		return
	}

	feature, err := web.DB.FindFeature(user.GetID())
	if err != nil {
		logger.Info("feature not found")
		http.Error(w, "feature not found", http.StatusNotFound)
		return
	}

	asset := &ApiAssetHandler{
		WebRouter: web,
		Logger:    logger,
		Feature:   feature,

		Username:       props.Username,
		UserID:         user.GetID(),
		Subdomain:      subdomain,
		ProjectDir:     projectDir,
		Filepath:       fname,
		Bucket:         bucket,
		ImgProcessOpts: opts,
	}

	asset.ServeHTTP(w, r)
}

func GetSubdomainFromRequest(r *http.Request, domain, prefix string) string {
	hostDomain := strings.ToLower(strings.Split(r.Host, ":")[0])
	appDomain := strings.ToLower(strings.Split(domain, ":")[0])

	if hostDomain != appDomain {
		if strings.Contains(hostDomain, appDomain) {
			subdomain := strings.TrimSuffix(hostDomain, fmt.Sprintf(".%s", appDomain))
			return subdomain
		} else {
			subdomain := GetCustomDomain(hostDomain, prefix)
			return subdomain
		}
	}

	return ""
}

func (web *WebRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	subdomain := GetSubdomainFromRequest(r, web.Domain, web.TxtPrefix)
	if web.RootRouter == nil || web.UserRouter == nil {
		web.Logger.Error("routers not initialized")
		http.Error(w, "routers not initialized", http.StatusInternalServerError)
		return
	}

	var router *http.ServeMux
	if subdomain == "" {
		router = web.RootRouter
	} else {
		router = web.UserRouter
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, CtxSubdomainKey{}, subdomain)
	router.ServeHTTP(w, r.WithContext(ctx))
}
