package minimal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/picosh/pgs"
	wsh "github.com/picosh/pico/wish"
	"github.com/picosh/send/auth"
	"github.com/picosh/send/list"
	wishrsync "github.com/picosh/send/protocols/rsync"
	"github.com/picosh/send/protocols/scp"
	"github.com/picosh/send/protocols/sftp"
	"github.com/picosh/send/proxy"
	"github.com/picosh/utils"
)

func createRouter(handler *pgs.UploadAssetHandler) proxy.Router {
	return func(sh ssh.Handler, s ssh.Session) []wish.Middleware {
		return []wish.Middleware{
			list.Middleware(handler),
			scp.Middleware(handler),
			wishrsync.Middleware(handler),
			auth.Middleware(handler),
			pgs.WishMiddleware(handler),
			wsh.LogMiddleware(handler.GetLogger()),
		}
	}
}

func withProxy(handler *pgs.UploadAssetHandler, otherMiddleware ...wish.Middleware) ssh.Option {
	return func(server *ssh.Server) error {
		err := sftp.SSHOption(handler)(server)
		if err != nil {
			return err
		}

		return proxy.WithProxy(createRouter(handler), otherMiddleware...)(server)
	}
}

func StartMinimalSshServer(cfg *pgs.ConfigSite, killCh chan error) {
	logger := cfg.Logger
	ctx := context.Background()
	defer ctx.Done()
	cacheClearingQueue := make(chan string, 100)
	handler := pgs.NewUploadAssetHandler(cfg, cacheClearingQueue)

	srv, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%s", cfg.SshHost, cfg.SshPort)),
		wish.WithHostKeyPath("ssh_data/term_info_ed25519"),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			pubkey := utils.KeyForKeyText(key)
			user, err := cfg.DB.FindUserByPubkey(ctx.User(), pubkey)
			if err != nil {
				return false
			}
			// the ssh app uses `user_id` to determine the current user
			// for the running session and is required
			pgs.SetUserIdForSession(ctx, user.GetID())
			return true
		}),
		withProxy(handler),
	)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	logger.Info(
		"starting ssh server",
		"host", cfg.SshHost,
		"port", cfg.SshPort,
	)
	go func() {
		if err = srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			logger.Error("serve", "err", err.Error())
			done <- nil
		}
	}()

	select {
	case <-done:
		logger.Info("stopping ssh server")
		os.Exit(0)
	case <-killCh:
		logger.Info("stopping ssh server")
	}
}
