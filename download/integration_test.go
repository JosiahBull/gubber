package download

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/go-github/github"
)

func TestDownloadRepo_CommandInjectionGuard(t *testing.T) {
	d := NewDownloader(context.Background(), strPtr("fake-token"))
	loc := t.TempDir()

	badNames := []string{"repo;rm -rf /", "repo|cat /etc/passwd", "repo&echo pwned"}
	for _, name := range badNames {
		owner := "org"
		fullName := owner + "/" + name
		repo := &github.Repository{
			Name:     &name,
			FullName: &fullName,
			Owner:    &github.User{Login: &owner},
		}
		err := d.DownloadRepo(repo, &loc)
		if err == nil {
			t.Errorf("expected error for repo name %q, got nil", name)
		}
	}
}

func TestDownloadRepo_EmptyName(t *testing.T) {
	d := NewDownloader(context.Background(), strPtr("fake-token"))
	loc := t.TempDir()

	name := ""
	fullName := ""
	owner := "org"
	repo := &github.Repository{
		Name:     &name,
		FullName: &fullName,
		Owner:    &github.User{Login: &owner},
	}

	err := d.DownloadRepo(repo, &loc)
	if err == nil {
		t.Error("expected error for empty repo name")
	}
}

func TestDownloadRepo_Success(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	// Create a local bare repo to serve as the "remote"
	srcDir := t.TempDir()
	workDir := filepath.Join(srcDir, "work")

	// We will create the bare repo at a path that matches the format string:
	// fmt.Sprintf(cloneBaseURL, token, fullName) should resolve to the bare repo path.
	// token = "", fullName = "org/repo", cloneBaseURL = srcDir + "/%s%s.git"
	// Result: srcDir + "/" + "" + "org/repo" + ".git" = srcDir + "/org/repo.git"
	owner := "org"
	name := "repo"
	fullName := owner + "/" + name
	bareDir := filepath.Join(srcDir, owner, name+".git")

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	os.MkdirAll(workDir, 0755)
	os.MkdirAll(filepath.Dir(bareDir), 0755)
	run(workDir, "git", "init")
	run(workDir, "git", "checkout", "-b", "main")
	os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# test"), 0644)
	run(workDir, "git", "add", ".")
	run(workDir, "git", "commit", "-m", "initial")
	run(srcDir, "git", "clone", "--bare", workDir, bareDir)

	// Build a Downloader that points at the local bare repo
	token := ""
	d := &Downloader{
		ctx:          context.Background(),
		token:        token,
		cloneBaseURL: srcDir + "/%s%s.git",
	}

	destDir := t.TempDir()
	repo := &github.Repository{
		Name:     &name,
		FullName: &fullName,
		Owner:    &github.User{Login: &owner},
	}

	err := d.DownloadRepo(repo, &destDir)
	if err != nil {
		t.Fatalf("DownloadRepo returned unexpected error: %v", err)
	}

	// The bundle should exist at destDir/org/repo.bundle
	bundlePath := filepath.Join(destDir, owner, name+".bundle")
	if !Exists(bundlePath) {
		t.Fatal("bundle was not created")
	}

	info, err := os.Stat(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("bundle file is empty")
	}

	// The cloned .git directory should have been cleaned up
	clonedGitDir := filepath.Join(destDir, owner, name+".git")
	if Exists(clonedGitDir) {
		t.Error("cloned .git directory was not cleaned up")
	}
}

func TestDownloadRepo_CloneFailure(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	token := ""
	d := &Downloader{
		ctx:          context.Background(),
		token:        token,
		cloneBaseURL: "/nonexistent/%s%s.git",
	}

	owner := "org"
	name := "repo"
	fullName := owner + "/" + name
	repo := &github.Repository{
		Name:     &name,
		FullName: &fullName,
		Owner:    &github.User{Login: &owner},
	}

	destDir := t.TempDir()
	err := d.DownloadRepo(repo, &destDir)
	if err == nil {
		t.Fatal("expected error when cloning from non-existent path, got nil")
	}
}

func TestDownloadRepos_EmptyList(t *testing.T) {
	d := NewDownloader(context.Background(), strPtr("fake"))
	loc := t.TempDir()
	err := d.DownloadRepos([]*github.Repository{}, &loc)
	if err == nil {
		t.Error("expected error for empty repo list")
	}
}
