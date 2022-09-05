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
	"strconv"
	"syscall"
	"time"

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
func (d *Downloader) DownloadRepos(repos []*github.Repository, location *string) error {
	var errCount uint16 = 0

	if len(repos) == 0 {
		return errors.New("no repos to download")
	}
	for _, repo := range repos {
		// try downloading the repo, if it fails, try again up to 20 times
		for i := 0; i < 20; i++ {
			err := d.DownloadRepo(repo, location)
			if err != nil {
				errCount++
				// if error count is greater than 20, fail out
				if errCount > 20 {
					return fmt.Errorf("failed to download repo %s due to error %w", repo.GetFullName(), err)
				}
				//wait 10 seconds before trying again
				time.Sleep(10 * time.Second)
				continue
			}
			break
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

	// increment all backups by one, starting from T-backup_limit and working down
	for i := backups_limit; i > 0; i-- {
		// if the backup exists, increment it
		if Exists(*existing_path + "/T-" + strconv.Itoa(i)) {
			err = os.Rename(*existing_path+"/T-"+strconv.Itoa(i), *existing_path+"/T-"+strconv.Itoa(i+1))
			if err != nil {
				return fmt.Errorf("failed to increment backup %d due to error %w", i, err)
			}
		}
	}

	// move the new repos to the existing path at T-0
	err = MoveFolder(temp_path, *existing_path+"/T-0")
	if err != nil {
		return fmt.Errorf("failed to move new repos to existing path due to error %w", err)
	}

	// if a file exists in the older backup, but not the newer backup, move it to the newer backup
	// if a file exists in both, generate a diff and replace the older backup with a .patch file
	for i := backups_limit + 1; i > 1; i-- {
		// if the backup exists, increment it
		if Exists(*existing_path + "/T-" + strconv.Itoa(i)) {
			// get a list of all files in the backup
			files, err := ioutil.ReadDir(*existing_path + "/T-" + strconv.Itoa(i))
			if err != nil {
				return fmt.Errorf("failed to get list of files in backup %d due to error %w", i, err)
			}

			// for each file, check if it exists in the older backup
			for _, file := range files {
				if Exists(*existing_path + "/T-" + strconv.Itoa(i-1) + "/" + file.Name()) {
					// if it does, generate a diff and replace the older backup with a .patch file
					//TODO: generate diff and replace patch file
				} else {
					// if it doesn't, move it to the newer backup
					err = os.Rename(*existing_path+"/T-"+strconv.Itoa(i)+"/"+file.Name(), *existing_path+"/T-"+strconv.Itoa(i-1)+"/"+file.Name())
					if err != nil {
						return fmt.Errorf("failed to move file %s from backup %d to backup %d due to error %w", file.Name(), i, i-1, err)
					}
				}
			}
		}
	}

	// try to delete T-backup_limit if it exists
	if Exists(*existing_path + "/T-" + strconv.Itoa(backups_limit)) {
		err = os.RemoveAll(*existing_path + "/T-" + strconv.Itoa(backups_limit))
		if err != nil {
			return fmt.Errorf("failed to delete backup %d due to error %w", backups_limit, err)
		}
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
