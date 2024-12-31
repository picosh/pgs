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
	"time"

	_ "net/http/pprof"

	"github.com/darkweak/souin/pkg/middleware"
	"github.com/darkweak/storages/core"
	"github.com/gorilla/feeds"
	"github.com/picosh/pgs/db"
	"github.com/picosh/pgs/storage"
	sst "github.com/picosh/pobj/storage"
	"google.golang.org/protobuf/proto"
)

func GetSubdomain(r *http.Request) string {
	return r.Context().Value(CtxSubdomainKey{}).(string)
}

func GetCustomDomain(host string, space string) string {
	txt := fmt.Sprintf("_%s.%s", space, host)
	records, err := net.LookupTXT(txt)
	if err != nil {
		return ""
	}

	for _, v := range records {
		return strings.TrimSpace(v)
	}

	return ""
}

type CachedHttp struct {
	handler *middleware.SouinBaseHandler
	routes  *WebRouter
}

func (c *CachedHttp) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	err := c.handler.ServeHTTP(writer, req, func(w http.ResponseWriter, r *http.Request) error {
		c.routes.ServeHTTP(w, r)
		return nil
	})
	if err != nil {
		c.routes.Logger.Error("serve http", "err", err)
	}
}

type HasPerm = func(proj *db.Project) bool

type WebRouter struct {
	Cfg        *ConfigSite
	Logger     *slog.Logger
	Dbpool     db.DB
	Storage    storage.StorageServe
	RootRouter *http.ServeMux
	UserRouter *http.ServeMux
}

func NewWebRouter(cfg *ConfigSite, logger *slog.Logger, dbpool db.DB, st storage.StorageServe) *WebRouter {
	router := &WebRouter{
		Cfg:     cfg,
		Logger:  logger,
		Dbpool:  dbpool,
		Storage: st,
	}
	router.initRouters()
	return router
}

func (web *WebRouter) initRouters() {
	// ensure legacy router is disabled
	// GODEBUG=httpmuxgo121=0

	// root domain
	rootRouter := http.NewServeMux()
	rootRouter.HandleFunc("GET /check", web.checkHandler)
	rootRouter.Handle("GET /main.css", web.serveFile("main.css", "text/css"))
	rootRouter.Handle("GET /favicon-16x16.png", web.serveFile("favicon-16x16.png", "image/png"))
	rootRouter.Handle("GET /apple-touch-icon.png", web.serveFile("apple-touch-icon.png", "image/png"))
	rootRouter.Handle("GET /favicon.ico", web.serveFile("favicon.ico", "image/x-icon"))
	rootRouter.Handle("GET /robots.txt", web.serveFile("robots.txt", "text/plain"))

	rootRouter.Handle("GET /rss/updated", web.createRssHandler("updated_at"))
	rootRouter.Handle("GET /rss", web.createRssHandler("created_at"))
	rootRouter.Handle("GET /{$}", web.createPageHandler("html/marketing.page.tmpl"))
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

func (web *WebRouter) createPageHandler(fname string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := web.Logger
		cfg := web.Cfg
		ts, err := RenderTemplate(cfg, []string{fname})

		if err != nil {
			logger.Error(
				"could not render template",
				"fname", fname,
				"err", err.Error(),
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = ts.Execute(w, nil)
		if err != nil {
			logger.Error(
				"could not execute template",
				"fname", fname,
				"err", err.Error(),
			)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (web *WebRouter) checkHandler(w http.ResponseWriter, r *http.Request) {
	dbpool := web.Dbpool
	cfg := web.Cfg
	logger := web.Logger

	hostDomain := r.URL.Query().Get("domain")
	appDomain := strings.Split(cfg.Domain, ":")[0]

	if !strings.Contains(hostDomain, appDomain) {
		subdomain := GetCustomDomain(hostDomain, cfg.Space)
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

		u, err := dbpool.FindUserForName(props.Username)
		if err != nil {
			logger.Error("could not find user", "err", err.Error())
			w.WriteHeader(http.StatusNotFound)
			return
		}

		logger = logger.With(
			"user", u.Name,
			"project", props.ProjectName,
		)
		p, err := dbpool.FindProjectByName(u.ID, props.ProjectName)
		if err != nil {
			logger.Error(
				"could not find project for user",
				"user", u.Name,
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

func (web *WebRouter) cacheMgmt(ctx context.Context, httpCache *middleware.SouinBaseHandler) {
	storer := httpCache.Storers[0]
	drain := createSubCacheDrain(ctx, web.Logger)

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

func (web *WebRouter) createRssHandler(by string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbpool := web.Dbpool
		logger := web.Logger
		cfg := web.Cfg

		pager, err := dbpool.FindAllProjects(&db.Pager{Num: 100, Page: 0}, by)
		if err != nil {
			logger.Error("could not find projects", "err", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		feed := &feeds.Feed{
			Title:       fmt.Sprintf("%s discovery feed %s", cfg.Domain, by),
			Link:        &feeds.Link{Href: "/"},
			Description: fmt.Sprintf("%s projects %s", cfg.Domain, by),
			Author:      &feeds.Author{Name: cfg.Domain},
			Created:     time.Now(),
		}

		var feedItems []*feeds.Item
		for _, project := range pager.Data {
			realUrl := strings.TrimSuffix(
				cfg.AssetURL(project.Username, project.Name, ""),
				"/",
			)
			uat := project.UpdatedAt.Unix()
			id := realUrl
			title := fmt.Sprintf("%s-%s", project.Username, project.Name)
			if by == "updated_at" {
				id = fmt.Sprintf("%s:%d", realUrl, uat)
				title = fmt.Sprintf("%s - %d", title, uat)
			}

			item := &feeds.Item{
				Id:          id,
				Title:       title,
				Link:        &feeds.Link{Href: realUrl},
				Content:     fmt.Sprintf(`<a href="%s">%s</a>`, realUrl, realUrl),
				Created:     *project.CreatedAt,
				Updated:     *project.CreatedAt,
				Description: "",
				Author:      &feeds.Author{Name: project.Username},
			}

			feedItems = append(feedItems, item)
		}
		feed.Items = feedItems

		rss, err := feed.ToAtom()
		if err != nil {
			logger.Error("could not convert feed to atom", "err", err.Error())
			http.Error(w, "Could not generate atom rss feed", http.StatusInternalServerError)
		}

		w.Header().Add("Content-Type", "application/atom+xml")
		_, err = w.Write([]byte(rss))
		if err != nil {
			logger.Error("http write failed", "err", err.Error())
		}
	}
}

func (web *WebRouter) Perm(proj *db.Project) bool {
	return proj.Acl.Type == "public"
}

var imgRegex = regexp.MustCompile("(.+.(?:jpg|jpeg|png|gif|webp|svg))(/.+)")

func (web *WebRouter) AssetRequest(w http.ResponseWriter, r *http.Request) {
	fname := r.PathValue("fname")
	if imgRegex.MatchString(fname) {
		web.ImageRequest(w, r)
		return
	}
	web.ServeAsset(fname, nil, false, web.Perm, w, r)
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

	web.ServeAsset(fname, opts, false, web.Perm, w, r)
}

func (web *WebRouter) ServeAsset(fname string, opts *storage.ImgProcessOpts, fromImgs bool, hasPerm HasPerm, w http.ResponseWriter, r *http.Request) {
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

	user, err := web.Dbpool.FindUserForName(props.Username)
	if err != nil {
		logger.Info("user not found")
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	logger = logger.With(
		"userId", user.ID,
	)

	projectID := ""
	// TODO: this could probably be cleaned up more
	// imgs wont have a project directory
	projectDir := ""
	var bucket sst.Bucket
	// imgs has a different bucket directory
	if fromImgs {
		bucket, err = web.Storage.GetBucket(GetImgsBucketName(user.ID))
	} else {
		bucket, err = web.Storage.GetBucket(GetAssetBucketName(user.ID))
		project, err := web.Dbpool.FindProjectByName(user.ID, props.ProjectName)
		if err != nil {
			logger.Info("project not found")
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}

		logger = logger.With(
			"projectId", project.ID,
			"project", project.Name,
		)

		if project.Blocked != "" {
			logger.Error("project has been blocked")
			http.Error(w, project.Blocked, http.StatusForbidden)
			return
		}

		projectID = project.ID
		projectDir = project.ProjectDir
		if !hasPerm(project) {
			http.Error(w, "You do not have access to this site", http.StatusUnauthorized)
			return
		}
	}

	if err != nil {
		logger.Info("bucket not found")
		http.Error(w, "bucket not found", http.StatusNotFound)
		return
	}

	hasPicoPlus := web.Dbpool.HasFeatureForUser(user.ID, "plus")

	asset := &ApiAssetHandler{
		WebRouter: web,
		Logger:    logger,

		Username:       props.Username,
		UserID:         user.ID,
		Subdomain:      subdomain,
		ProjectDir:     projectDir,
		Filepath:       fname,
		Bucket:         bucket,
		ImgProcessOpts: opts,
		ProjectID:      projectID,
		HasPicoPlus:    hasPicoPlus,
	}

	asset.ServeHTTP(w, r)
}

func GetSubdomainFromRequest(r *http.Request, domain, space string) string {
	hostDomain := strings.ToLower(strings.Split(r.Host, ":")[0])
	appDomain := strings.ToLower(strings.Split(domain, ":")[0])

	if hostDomain != appDomain {
		if strings.Contains(hostDomain, appDomain) {
			subdomain := strings.TrimSuffix(hostDomain, fmt.Sprintf(".%s", appDomain))
			return subdomain
		} else {
			subdomain := GetCustomDomain(hostDomain, space)
			return subdomain
		}
	}

	return ""
}

func (web *WebRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	subdomain := GetSubdomainFromRequest(r, web.Cfg.Domain, web.Cfg.Space)
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

type CompatLogger struct {
	logger *slog.Logger
}

func (cl *CompatLogger) marshall(int ...interface{}) string {
	res := ""
	for _, val := range int {
		switch r := val.(type) {
		case string:
			res += " " + r
		}
	}
	return res
}
func (cl *CompatLogger) DPanic(int ...interface{}) {
	cl.logger.Error("panic", "output", cl.marshall(int))
}
func (cl *CompatLogger) DPanicf(st string, int ...interface{}) {
	cl.logger.Error(fmt.Sprintf(st, int...))
}
func (cl *CompatLogger) Debug(int ...interface{}) {
	cl.logger.Debug("debug", "output", cl.marshall(int))
}
func (cl *CompatLogger) Debugf(st string, int ...interface{}) {
	cl.logger.Debug(fmt.Sprintf(st, int...))
}
func (cl *CompatLogger) Error(int ...interface{}) {
	cl.logger.Error("error", "output", cl.marshall(int))
}
func (cl *CompatLogger) Errorf(st string, int ...interface{}) {
	cl.logger.Error(fmt.Sprintf(st, int...))
}
func (cl *CompatLogger) Fatal(int ...interface{}) {
	cl.logger.Error("fatal", "outpu", cl.marshall(int))
}
func (cl *CompatLogger) Fatalf(st string, int ...interface{}) {
	cl.logger.Error(fmt.Sprintf(st, int...))
}
func (cl *CompatLogger) Info(int ...interface{}) {
	cl.logger.Info("info", "output", cl.marshall(int))
}
func (cl *CompatLogger) Infof(st string, int ...interface{}) {
	cl.logger.Info(fmt.Sprintf(st, int...))
}
func (cl *CompatLogger) Panic(int ...interface{}) {
	cl.logger.Error("panic", "output", cl.marshall(int))
}
func (cl *CompatLogger) Panicf(st string, int ...interface{}) {
	cl.logger.Error(fmt.Sprintf(st, int...))
}
func (cl *CompatLogger) Warn(int ...interface{}) {
	cl.logger.Warn("warn", "output", cl.marshall(int))
}
func (cl *CompatLogger) Warnf(st string, int ...interface{}) {
	cl.logger.Warn(fmt.Sprintf(st, int...))
}
