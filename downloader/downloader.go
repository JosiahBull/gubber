package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

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
	temp_path, err := ioutil.TempDir("", "gubber")
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
	err = MoveFolder(temp_path, *existing_path+"/backT-0")
	if err != nil {
		return fmt.Errorf("failed to move temporary location to repo location due to error %w", err)
	}

	return nil
}

func MoveFolder(sourcePath, destPath string) error {
	// copy the directories
	err := CopyDirectory(sourcePath, destPath)
	if err != nil {
		return fmt.Errorf("failed to copy folder due to error %w", err)
	}

	// remove the source folder
	err = os.RemoveAll(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to remove source folder due to error %w", err)
	}
	return nil
}

func CopyDirectory(scrDir, dest string) error {
	entries, err := os.ReadDir(scrDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(scrDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		stat, ok := fileInfo.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("failed to get raw syscall.Stat_t data for '%s'", sourcePath)
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := CreateIfNotExists(destPath, 0755); err != nil {
				return err
			}
			if err := CopyDirectory(sourcePath, destPath); err != nil {
				return err
			}
		case os.ModeSymlink:
			if err := CopySymLink(sourcePath, destPath); err != nil {
				return err
			}
		default:
			if err := Copy(sourcePath, destPath); err != nil {
				return err
			}
		}

		if err := os.Lchown(destPath, int(stat.Uid), int(stat.Gid)); err != nil {
			return err
		}

		fInfo, err := entry.Info()
		if err != nil {
			return err
		}

		isSymlink := fInfo.Mode()&os.ModeSymlink != 0
		if !isSymlink {
			if err := os.Chmod(destPath, fInfo.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func Copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}

	defer out.Close()

	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer in.Close()
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

func Exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func CreateIfNotExists(dir string, perm os.FileMode) error {
	if Exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}

func CopySymLink(source, dest string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}
	return os.Symlink(link, dest)
}
