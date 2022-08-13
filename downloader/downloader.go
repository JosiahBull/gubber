package downloader

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
)

type Downloader struct {
	ctx   context.Context
	token string
}

func NewDownloader(ctx context.Context, token *string) *Downloader {
	return &Downloader{
		ctx:   ctx,
		token: *token,
	}
}

// DownloadRepo will download a repo from github, saving it in the preconfigured location, under org/repo-name
func (d *Downloader) DownloadRepo(repo *github.Repository, location *string) error {
	if repo.GetFullName() == "" {
		return errors.New("repo name is empty")
	}

	// create the org folder if it doesn't exist
	fmt.Println("Creating folder:", *location+"/"+repo.GetFullName())
	org_folder := *location + "/" + repo.GetOwner().GetLogin()

	err := os.MkdirAll(org_folder, 0755)
	if err != nil {
		return fmt.Errorf("failed to create org folder due to error %w", err)
	}

	// download the repo
	fmt.Println("Downloading:", repo.GetFullName())
	cmd := exec.CommandContext(d.ctx, "git", "clone", "--mirror", "https://"+d.token+"@github.com/"+repo.GetFullName()+".git", org_folder+"/"+repo.GetName()+".git")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to download repo due to error %w\nstdout + stderr: %s", err, output)
	}

	// bundle the repo
	fmt.Println("Bundling:", repo.GetFullName())
	cmd = exec.CommandContext(d.ctx, "git", "bundle", "create", repo.GetName()+".bundle", "--all")
	cmd.Dir = org_folder + "/" + repo.GetName() + ".git"

	// run command getting stdout and stderr
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to bundle repo due to error %w\nstdout + stderr: %s", err, output)
	}

	// move the bundle to the download location
	fmt.Println("Moving:", repo.GetFullName())
	err = os.Rename(org_folder+"/"+repo.GetName()+".git/"+repo.GetName()+".bundle", org_folder+"/"+repo.GetName()+".bundle")
	if err != nil {
		return fmt.Errorf("failed to move bundle to download location due to error %w", err)
	}

	// delete the .git repo
	fmt.Println("Cleaning:", repo.GetFullName())
	err = os.RemoveAll(org_folder + "/" + repo.GetName() + ".git")
	if err != nil {
		return fmt.Errorf("failed to clean repo due to error %w", err)
	}

	return nil
}

// DownloadRepos will download all repos from github, saving them in the preconfigured location, under org/repo-name
// it will download using multiple go routines to download up to 8 repositories at a time
func (d *Downloader) DownloadRepos(repos []*github.Repository, location *string) error {
	if len(repos) == 0 {
		return errors.New("no repos to download")
	}
	for _, repo := range repos {
		err := d.DownloadRepo(repo, location)
		if err != nil {
			return fmt.Errorf("failed to download repo %s due to error %w", repo.GetFullName(), err)
		}
	}
	return nil

}

func (d *Downloader) MigrateRepos(new_repos []*github.Repository, existing_path *string, backups_limit int) error {
	// create temporary location to download repos
	temp_path, err := ioutil.TempDir("", "downloader")
	if err != nil {
		return fmt.Errorf("failed to create temporary location due to error %w", err)
	}
	defer os.RemoveAll(temp_path)

	// download all new repos
	err = d.DownloadRepos(new_repos, &temp_path)
	if err != nil {
		return fmt.Errorf("failed to download new repos due to error %w", err)
	}

	// for every folder called "backT-x" in the existing location, increment x in it's name by one and move it
	items, err := ioutil.ReadDir(*existing_path)
	if err != nil {
		return fmt.Errorf("failed to read existing location due to error %w", err)
	}

	// sort items alphabetically
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name() < items[j].Name()
	})

	// rename old folders
	for _, item := range items {
		if item.IsDir() {
			if strings.HasPrefix(item.Name(), "backT-") {
				file_number, err := strconv.Atoi(item.Name()[7:])
				if err != nil {
					return fmt.Errorf("failed to convert folder name to int due to error %w", err)
				}

				if file_number >= backups_limit {
					err = os.RemoveAll(*existing_path + "/" + item.Name())
					if err != nil {
						return fmt.Errorf("failed to remove old folder due to error %w", err)
					}
				}

				new_name := "backT-" + strconv.Itoa(file_number+1)
				err = os.Rename(*existing_path+"/"+item.Name(), *existing_path+"/"+new_name)
				if err != nil {
					return fmt.Errorf("failed to rename folder due to error %w", err)
				}
			}
		}
	}

	// move the temporary location to the repo location, naming it "backT-0"
	err = os.Rename(temp_path, *existing_path+"/backT-0")
	if err != nil {
		return fmt.Errorf("failed to move temporary location to repo location due to error %w", err)
	}

	return nil
}
