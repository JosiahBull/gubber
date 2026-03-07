package download

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-github/github"
)

// mockDownloader creates fake bundle files in the download location to simulate DownloadRepos.
type mockDownloader struct {
	repos []*github.Repository
}

func (m *mockDownloader) DownloadRepos(repos []*github.Repository, location *string) error {
	m.repos = repos
	for _, repo := range repos {
		orgDir := filepath.Join(*location, repo.GetOwner().GetLogin())
		if err := os.MkdirAll(orgDir, 0755); err != nil {
			return err
		}
		bundlePath := filepath.Join(orgDir, repo.GetName()+".bundle")
		if err := os.WriteFile(bundlePath, []byte("fake-bundle-"+repo.GetFullName()), 0644); err != nil {
			return err
		}
	}
	return nil
}

func strPtr(s string) *string { return &s }

func makeRepo(owner, name string) *github.Repository {
	fullName := owner + "/" + name
	return &github.Repository{
		Name:     &name,
		FullName: &fullName,
		Owner:    &github.User{Login: &owner},
	}
}

func TestMigrateRepos_FirstRun(t *testing.T) {
	baseDir := t.TempDir()
	tmpDir := t.TempDir()
	existingPath := filepath.Join(baseDir, "backups")
	if err := os.MkdirAll(existingPath, 0755); err != nil {
		t.Fatal(err)
	}

	repos := []*github.Repository{makeRepo("org1", "repo1")}
	dl := &mockDownloader{}

	err := MigrateReposWithDownloader(dl, repos, &existingPath, 3, &tmpDir)
	if err != nil {
		t.Fatalf("MigrateRepos() error: %v", err)
	}

	// T-0 should exist with the bundle
	bundlePath := filepath.Join(existingPath, "T-0", "org1", "repo1.bundle")
	if !Exists(bundlePath) {
		t.Errorf("expected bundle at %s", bundlePath)
	}

	got, _ := os.ReadFile(bundlePath)
	if string(got) != "fake-bundle-org1/repo1" {
		t.Errorf("bundle content = %q", got)
	}
}

func TestMigrateRepos_RotatesBackups(t *testing.T) {
	baseDir := t.TempDir()
	tmpDir := t.TempDir()
	existingPath := filepath.Join(baseDir, "backups")

	// Pre-create T-0 with an existing bundle
	t0OrgDir := filepath.Join(existingPath, "T-0", "org1")
	if err := os.MkdirAll(t0OrgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(t0OrgDir, "old.bundle"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	repos := []*github.Repository{makeRepo("org1", "repo1")}
	dl := &mockDownloader{}

	err := MigrateReposWithDownloader(dl, repos, &existingPath, 3, &tmpDir)
	if err != nil {
		t.Fatalf("MigrateRepos() error: %v", err)
	}

	// New T-0 should have the new bundle
	newBundle := filepath.Join(existingPath, "T-0", "org1", "repo1.bundle")
	if !Exists(newBundle) {
		t.Error("new bundle not at T-0")
	}

	// old.bundle should be promoted from T-1 to T-0 (since it's missing from T-0)
	promotedBundle := filepath.Join(existingPath, "T-0", "org1", "old.bundle")
	if !Exists(promotedBundle) {
		t.Error("old.bundle not promoted from T-1 to T-0")
	}
}

func TestMigrateRepos_PromotesMissingFiles(t *testing.T) {
	baseDir := t.TempDir()
	tmpDir := t.TempDir()
	existingPath := filepath.Join(baseDir, "backups")

	// Pre-create T-0 with repo_old (will become T-1 after rotation)
	t0OrgDir := filepath.Join(existingPath, "T-0", "org1")
	if err := os.MkdirAll(t0OrgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(t0OrgDir, "repo_old.bundle"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	// New download only has repo_new — repo_old should be promoted from T-1 to T-0
	repos := []*github.Repository{makeRepo("org1", "repo_new")}
	dl := &mockDownloader{}

	err := MigrateReposWithDownloader(dl, repos, &existingPath, 3, &tmpDir)
	if err != nil {
		t.Fatalf("MigrateRepos() error: %v", err)
	}

	// repo_old should be promoted to T-0 from T-1
	promoted := filepath.Join(existingPath, "T-0", "org1", "repo_old.bundle")
	if !Exists(promoted) {
		t.Error("repo_old.bundle was not promoted from T-1 to T-0")
	}

	// repo_new should be in T-0
	newBundle := filepath.Join(existingPath, "T-0", "org1", "repo_new.bundle")
	if !Exists(newBundle) {
		t.Error("repo_new.bundle not at T-0")
	}
}

func TestMigrateRepos_DeletesOldestBackup(t *testing.T) {
	baseDir := t.TempDir()
	tmpDir := t.TempDir()
	existingPath := filepath.Join(baseDir, "backups")
	backupsLimit := 2

	// Pre-create T-0 and T-1 (at limit)
	for i := 0; i < backupsLimit; i++ {
		dir := filepath.Join(existingPath, "T-"+string(rune('0'+i)), "org1")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "repo.bundle"), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	repos := []*github.Repository{makeRepo("org1", "repo_new")}
	dl := &mockDownloader{}

	err := MigrateReposWithDownloader(dl, repos, &existingPath, backupsLimit, &tmpDir)
	if err != nil {
		t.Fatalf("MigrateRepos() error: %v", err)
	}

	// T-{backupsLimit} should be deleted
	oldest := filepath.Join(existingPath, "T-2")
	if Exists(oldest) {
		t.Errorf("T-%d should have been deleted", backupsLimit)
	}
}

func TestMigrateRepos_CleansEmptyOrgDirs(t *testing.T) {
	baseDir := t.TempDir()
	tmpDir := t.TempDir()
	existingPath := filepath.Join(baseDir, "backups")

	// Pre-create T-0 with org_old/only_repo.bundle (will become T-1 after rotation)
	t0OrgDir := filepath.Join(existingPath, "T-0", "org_old")
	if err := os.MkdirAll(t0OrgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(t0OrgDir, "only_repo.bundle"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Download a repo under a different org — only_repo.bundle will be promoted
	// from T-1/org_old/ to T-0/org_old/, leaving T-1/org_old/ empty
	repos := []*github.Repository{makeRepo("org_new", "repo_new")}
	dl := &mockDownloader{}

	err := MigrateReposWithDownloader(dl, repos, &existingPath, 3, &tmpDir)
	if err != nil {
		t.Fatalf("MigrateRepos() error: %v", err)
	}

	// only_repo.bundle should have been promoted from T-1 to T-0
	promoted := filepath.Join(existingPath, "T-0", "org_old", "only_repo.bundle")
	if !Exists(promoted) {
		t.Error("org_old/only_repo.bundle was not promoted to T-0")
	}

	// The newly downloaded repo should be in T-0
	newBundle := filepath.Join(existingPath, "T-0", "org_new", "repo_new.bundle")
	if !Exists(newBundle) {
		t.Error("org_new/repo_new.bundle not found in T-0")
	}

	// T-1/org_old/ should have been cleaned up because it is now empty
	t1OrgOld := filepath.Join(existingPath, "T-1", "org_old")
	if Exists(t1OrgOld) {
		t.Error("T-1/org_old/ should have been removed after promotion left it empty")
	}
}
