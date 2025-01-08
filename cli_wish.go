package pgs

import (
	"flag"
	"fmt"
	"strings"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/muesli/termenv"
	"github.com/picosh/pgs/db"
	"github.com/picosh/utils"
)

func flagSet(cmdName string, sesh ssh.Session) (*flag.FlagSet, *bool) {
	cmd := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	cmd.SetOutput(sesh)
	write := cmd.Bool("write", false, "apply changes")
	return cmd, write
}

func flagCheck(cmd *flag.FlagSet, posArg string, cmdArgs []string) bool {
	_ = cmd.Parse(cmdArgs)

	if posArg == "-h" || posArg == "--help" || posArg == "-help" {
		cmd.Usage()
		return false
	}
	return true
}

func getUser(s ssh.Session, dbpool db.DB) (db.User, error) {
	if s.PublicKey() == nil {
		return nil, fmt.Errorf("key not found")
	}

	key := utils.KeyForKeyText(s.PublicKey())

	user, err := dbpool.FindUserByPubkey(s.User(), key)
	if err != nil {
		return nil, err
	}

	if user.GetName() == "" {
		return nil, fmt.Errorf("must have username set")
	}

	return user, nil
}

func WishMiddleware(handler *UploadAssetHandler) wish.Middleware {
	dbpool := handler.Cfg.DB
	log := handler.Cfg.Logger
	store := handler.Cfg.Storage

	return func(next ssh.Handler) ssh.Handler {
		return func(sesh ssh.Session) {
			args := sesh.Command()
			if len(args) == 0 {
				next(sesh)
				return
			}

			// default width and height when no pty
			width := 100
			height := 24
			pty, _, ok := sesh.Pty()
			if ok {
				width = pty.Window.Width
				height = pty.Window.Height
			}

			user, err := getUser(sesh, dbpool)
			if err != nil {
				wish.Errorln(sesh, err)
				return
			}

			renderer := bm.MakeRenderer(sesh)
			renderer.SetColorProfile(termenv.TrueColor)

			opts := Cmd{
				Session: sesh,
				User:    user,
				Store:   store,
				Log:     log,
				Dbpool:  dbpool,
				Write:   false,
				Width:   width,
				Height:  height,
				Cfg:     handler.Cfg,
			}

			cmd := strings.TrimSpace(args[0])
			if len(args) == 1 {
				if cmd == "help" {
					opts.help()
					return
				} else if cmd == "ls" {
					err := opts.ls()
					opts.bail(err)
					return
				} else if cmd == "cache-all" {
					opts.Write = true
					err := opts.cacheAll()
					opts.notice()
					opts.bail(err)
					return
				} else {
					next(sesh)
					return
				}
			}

			projectName := strings.TrimSpace(args[1])
			cmdArgs := args[2:]
			log.Info(
				"pgs middleware detected command",
				"args", args,
				"cmd", cmd,
				"projectName", projectName,
				"cmdArgs", cmdArgs,
			)

			if cmd == "link" {
				linkCmd, write := flagSet("link", sesh)
				linkTo := linkCmd.String("to", "", "symbolic link to this project")
				if !flagCheck(linkCmd, projectName, cmdArgs) {
					return
				}
				opts.Write = *write

				if *linkTo == "" {
					err := fmt.Errorf(
						"must provide `--to` flag",
					)
					opts.bail(err)
					return
				}

				err := opts.link(projectName, *linkTo)
				opts.notice()
				if err != nil {
					opts.bail(err)
				}
				return
			} else if cmd == "unlink" {
				unlinkCmd, write := flagSet("unlink", sesh)
				if !flagCheck(unlinkCmd, projectName, cmdArgs) {
					return
				}
				opts.Write = *write

				err := opts.unlink(projectName)
				opts.notice()
				opts.bail(err)
				return
			} else if cmd == "depends" {
				err := opts.depends(projectName)
				opts.bail(err)
				return
			} else if cmd == "retain" {
				retainCmd, write := flagSet("retain", sesh)
				retainNum := retainCmd.Int("n", 3, "latest number of projects to keep")
				if !flagCheck(retainCmd, projectName, cmdArgs) {
					return
				}
				opts.Write = *write

				err := opts.prune(projectName, *retainNum)
				opts.notice()
				opts.bail(err)
				return
			} else if cmd == "prune" {
				pruneCmd, write := flagSet("prune", sesh)
				if !flagCheck(pruneCmd, projectName, cmdArgs) {
					return
				}
				opts.Write = *write

				err := opts.prune(projectName, 0)
				opts.notice()
				opts.bail(err)
				return
			} else if cmd == "rm" {
				rmCmd, write := flagSet("rm", sesh)
				if !flagCheck(rmCmd, projectName, cmdArgs) {
					return
				}
				opts.Write = *write

				err := opts.rm(projectName)
				opts.notice()
				opts.bail(err)
				return
			} else if cmd == "cache" {
				cacheCmd, write := flagSet("cache", sesh)
				if !flagCheck(cacheCmd, projectName, cmdArgs) {
					return
				}
				opts.Write = *write

				err := opts.cache(projectName)
				opts.notice()
				opts.bail(err)
				return
			} else {
				next(sesh)
				return
			}
		}
	}
}
