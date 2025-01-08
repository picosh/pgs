package pgs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/picosh/pgs/db"
	sst "github.com/picosh/pobj/storage"
	"github.com/picosh/utils"
)

func projectTable(projects []db.Project, width int) *table.Table {
	headers := []string{
		"Name",
		"Last Updated",
		"Links To",
		"ACL Type",
		"ACL",
		"Blocked",
	}
	data := [][]string{}
	for _, project := range projects {
		row := []string{
			project.GetName(),
			project.GetUpdatedAt().Format("2006-01-02 15:04:05"),
		}
		links := ""
		if project.GetProjectDir() != project.GetName() {
			links = project.GetProjectDir()
		}
		row = append(row, links)
		data = append(data, row)
	}

	t := table.New().
		Width(width).
		Headers(headers...).
		Rows(data...)
	return t
}

func getHelpText(userName string, width int) string {
	helpStr := "Commands: [help, stats, ls, rm, link, unlink, prune, retain, depends, acl, cache]\n"
	helpStr += "NOTICE: *must* append with `--write` for the changes to persist.\n"

	projectName := "projA"
	headers := []string{"Cmd", "Description"}
	data := [][]string{
		{
			"help",
			"prints this screen",
		},
		{
			"stats",
			"usage statistics",
		},
		{
			"ls",
			"lists projects",
		},
		{
			fmt.Sprintf("rm %s", projectName),
			fmt.Sprintf("delete %s", projectName),
		},
		{
			fmt.Sprintf("link %s --to projB", projectName),
			fmt.Sprintf("symbolic link `%s` to `projB`", projectName),
		},
		{
			fmt.Sprintf("unlink %s", projectName),
			fmt.Sprintf("removes symbolic link for `%s`", projectName),
		},
		{
			fmt.Sprintf("prune %s", projectName),
			fmt.Sprintf("removes projects that match prefix `%s`", projectName),
		},
		{
			fmt.Sprintf("retain %s", projectName),
			"alias to `prune` but keeps last N projects",
		},
		{
			fmt.Sprintf("depends %s", projectName),
			fmt.Sprintf("lists all projects linked to `%s`", projectName),
		},
		{
			fmt.Sprintf("acl %s", projectName),
			fmt.Sprintf("access control for `%s`", projectName),
		},
		{
			fmt.Sprintf("cache %s", projectName),
			fmt.Sprintf("clear http cache for `%s`", projectName),
		},
	}

	t := table.New().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		Headers(headers...).
		Rows(data...)

	helpStr += t.String()
	return helpStr
}

type Cmd struct {
	User    db.User
	Session utils.CmdSession
	Log     *slog.Logger
	Store   sst.ObjectStorage
	Dbpool  db.DB
	Write   bool
	Width   int
	Height  int
	Cfg     *ConfigSite
}

func (c *Cmd) output(out string) {
	_, _ = c.Session.Write([]byte(out + "\r\n"))
}

func (c *Cmd) error(err error) {
	_, _ = fmt.Fprint(c.Session.Stderr(), err, "\r\n")
	_ = c.Session.Exit(1)
	_ = c.Session.Close()
}

func (c *Cmd) bail(err error) {
	if err == nil {
		return
	}
	c.Log.Error(err.Error())
	c.error(err)
}

func (c *Cmd) notice() {
	if !c.Write {
		c.output("\nNOTICE: changes not commited, use `--write` to save operation")
	}
}

func (c *Cmd) RmProjectAssets(projectName string) error {
	bucketName := GetAssetBucketName(c.User.GetID())
	bucket, err := c.Store.GetBucket(bucketName)
	if err != nil {
		return err
	}
	c.output(fmt.Sprintf("removing project assets (%s)", projectName))

	fileList, err := c.Store.ListObjects(bucket, projectName+"/", true)
	if err != nil {
		return err
	}

	if len(fileList) == 0 {
		c.output(fmt.Sprintf("no assets found for project (%s)", projectName))
		return nil
	}
	c.output(fmt.Sprintf("found (%d) assets for project (%s), removing", len(fileList), projectName))

	for _, file := range fileList {
		intent := fmt.Sprintf("deleted (%s)", file.Name())
		c.Log.Info(
			"attempting to delete file",
			"user", c.User.GetName(),
			"bucket", bucket.Name,
			"filename", file.Name(),
		)
		if c.Write {
			err = c.Store.DeleteObject(
				bucket,
				filepath.Join(projectName, file.Name()),
			)
			if err == nil {
				c.output(intent)
			} else {
				return err
			}
		} else {
			c.output(intent)
		}
	}
	return nil
}

func (c *Cmd) help() {
	c.output(getHelpText(c.User.GetName(), c.Width))
}

func (c *Cmd) ls() error {
	projects, err := c.Dbpool.FindProjectsByUser(c.User.GetID())
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		c.output("no projects found")
	}

	t := projectTable(projects, c.Width)
	c.output(t.String())

	return nil
}

func (c *Cmd) unlink(projectName string) error {
	c.Log.Info("user running `unlink` command", "user", c.User.GetName(), "project", projectName)
	project, err := c.Dbpool.FindProjectByName(c.User.GetID(), projectName)
	if err != nil {
		return errors.Join(err, fmt.Errorf("project (%s) does not exit", projectName))
	}

	err = c.Dbpool.LinkToProject(c.User.GetID(), project.GetID(), project.GetName(), c.Write)
	if err != nil {
		return err
	}
	c.output(fmt.Sprintf("(%s) unlinked", project.GetName()))

	return nil
}

func (c *Cmd) link(projectName, linkTo string) error {
	c.Log.Info("user running `link` command", "user", c.User.GetName(), "project", projectName, "link", linkTo)

	projectDir := linkTo
	_, err := c.Dbpool.FindProjectByName(c.User.GetID(), linkTo)
	if err != nil {
		e := fmt.Errorf("(%s) project doesn't exist", linkTo)
		return e
	}

	project, err := c.Dbpool.FindProjectByName(c.User.GetID(), projectName)
	projectID := ""
	if err == nil {
		projectID = project.GetID()
		c.Log.Info("user already has project, updating", "user", c.User.GetName(), "project", projectName)
		err = c.Dbpool.LinkToProject(c.User.GetID(), project.GetID(), projectDir, c.Write)
		if err != nil {
			return err
		}
	} else {
		c.Log.Info("user has no project record, creating", "user", c.User.GetName(), "project", projectName)
		if !c.Write {
			out := fmt.Sprintf("(%s) cannot create a new project without `--write` permission, aborting", projectName)
			c.output(out)
			return nil
		}
		id, err := c.Dbpool.InsertProject(c.User.GetID(), projectName, projectName)
		if err != nil {
			return err
		}
		projectID = id
	}

	c.Log.Info("user linking", "user", c.User.GetName(), "project", projectName, "projectDir", projectDir)
	err = c.Dbpool.LinkToProject(c.User.GetID(), projectID, projectDir, c.Write)
	if err != nil {
		return err
	}

	out := fmt.Sprintf("(%s) might have orphaned assets, removing", projectName)
	c.output(out)

	err = c.RmProjectAssets(projectName)
	if err != nil {
		return err
	}

	out = fmt.Sprintf("(%s) now points to (%s)", projectName, linkTo)
	c.output(out)
	return nil
}

func (c *Cmd) depends(projectName string) error {
	projects, err := c.Dbpool.FindProjectLinks(c.User.GetID(), projectName)
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		out := fmt.Sprintf("no projects linked to (%s)", projectName)
		c.output(out)
		return nil
	}

	t := projectTable(projects, c.Width)
	c.output(t.String())

	return nil
}

// delete all the projects and associated assets matching prefix
// but keep the latest N records.
func (c *Cmd) prune(prefix string, keepNumLatest int) error {
	c.Log.Info("user running `clean` command", "user", c.User.GetName(), "prefix", prefix)
	c.output(fmt.Sprintf("searching for projects that match prefix (%s) and are not linked to other projects", prefix))

	if prefix == "" || prefix == "*" {
		e := fmt.Errorf("must provide valid prefix")
		return e
	}

	projects, err := c.Dbpool.FindProjectsByPrefix(c.User.GetID(), prefix)
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		c.output(fmt.Sprintf("no projects found matching prefix (%s)", prefix))
		return nil
	}

	rmProjects := []db.Project{}
	for _, project := range projects {
		links, err := c.Dbpool.FindProjectLinks(c.User.GetID(), project.GetName())
		if err != nil {
			return err
		}

		if len(links) == 0 {
			rmProjects = append(rmProjects, project)
		} else {
			out := fmt.Sprintf("project (%s) has (%d) projects linked to it, cannot prune", project.GetName(), len(links))
			c.output(out)
		}
	}

	goodbye := rmProjects
	if keepNumLatest > 0 {
		pmax := len(rmProjects) - (keepNumLatest)
		if pmax <= 0 {
			out := fmt.Sprintf(
				"no projects available to prune (retention policy: %d, total: %d)",
				keepNumLatest,
				len(rmProjects),
			)
			c.output(out)
			return nil
		}
		goodbye = rmProjects[:pmax]
	}

	for _, project := range goodbye {
		out := fmt.Sprintf("project (%s) is available to be pruned", project.GetName())
		c.output(out)
		err = c.RmProjectAssets(project.GetName())
		if err != nil {
			return err
		}

		out = fmt.Sprintf("(%s) removing", project.GetName())
		c.output(out)

		if c.Write {
			c.Log.Info("removing project", "project", project.GetName())
			err = c.Dbpool.RemoveProject(project.GetID())
			if err != nil {
				return err
			}
		}
	}

	c.output("\nsummary")
	c.output("=======")
	for _, project := range goodbye {
		c.output(fmt.Sprintf("project (%s) removed", project.GetName()))
	}

	return nil
}

func (c *Cmd) rm(projectName string) error {
	c.Log.Info("user running `rm` command", "user", c.User.GetName(), "project", projectName)
	project, err := c.Dbpool.FindProjectByName(c.User.GetID(), projectName)
	if err == nil {
		c.Log.Info("found project, checking dependencies", "project", projectName, "projectID", project.GetID())

		links, err := c.Dbpool.FindProjectLinks(c.User.GetID(), projectName)
		if err != nil {
			return err
		}

		if len(links) > 0 {
			e := fmt.Errorf("project (%s) has (%d) projects linking to it, cannot delete project until they have been unlinked or removed, aborting", projectName, len(links))
			return e
		}

		out := fmt.Sprintf("(%s) removing", project.GetName())
		c.output(out)
		if c.Write {
			c.Log.Info("removing project", "project", project.GetName())
			err = c.Dbpool.RemoveProject(project.GetID())
			if err != nil {
				return err
			}
		}
	} else {
		msg := fmt.Sprintf("(%s) project record not found for user (%s)", projectName, c.User.GetName())
		c.output(msg)
	}

	err = c.RmProjectAssets(projectName)
	return err
}

func (c *Cmd) cache(projectName string) error {
	c.Log.Info(
		"user running `cache` command",
		"user", c.User.GetName(),
		"project", projectName,
	)
	c.output(fmt.Sprintf("clearing http cache for %s", projectName))
	ctx := context.Background()
	defer ctx.Done()
	send := CreatePubCacheDrain(ctx, c.Log)
	if c.Write {
		surrogate := getSurrogateKey(c.User.GetName(), projectName)
		return purgeCache(c.Cfg, send, surrogate)
	}
	return nil
}

func (c *Cmd) cacheAll() error {
	feature, err := c.Dbpool.FindFeature(c.User.GetID())
	if err != nil {
		return err
	}
	if !slices.Contains(feature.GetPerms(), "admin") {
		return fmt.Errorf("must be admin to use this command")
	}

	c.Log.Info(
		"admin running `cache-all` command",
		"user", c.User.GetName(),
	)
	c.output("clearing http cache for all sites")
	if c.Write {
		ctx := context.Background()
		defer ctx.Done()
		send := CreatePubCacheDrain(ctx, c.Log)
		return purgeAllCache(c.Cfg, send)
	}
	return nil
}
