package minimal

import (
	"fmt"
	"net/http"

	"github.com/picosh/pgs"
)

func StartMinimalWebServer(cfg *pgs.ConfigSite) {
	logger := cfg.Logger
	routes := pgs.NewWebRouter(cfg)

	portStr := fmt.Sprintf(":%s", cfg.WebPort)
	logger.Info(
		"starting web server",
		"port", cfg.WebPort,
		"domain", cfg.Domain,
	)
	err := http.ListenAndServe(portStr, routes)
	logger.Error(
		"listen and serve",
		"err", err.Error(),
	)
}
