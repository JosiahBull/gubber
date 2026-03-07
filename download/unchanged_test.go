package download

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-github/github"
)

type mockLister struct {
	commits []*string
	err     error
}

func (m *mockLister) GetOrgs() ([]*github.Organization, error) { return nil, nil }
func (m *mockLister) GetRepos() ([]*github.Repository, error)  { return nil, nil }
func (m *mockLister) GetOrgRepos(_ *github.Organization) ([]*github.Repository, error) {
	return nil, nil
}
func (m *mockLister) RemoveEmptyRepos(repos []*github.Repository) ([]*github.Repository, error) {
	return repos, nil
}
func (m *mockLister) GetLastCommits(_ []*github.Repository) ([]*string, error) {
	return m.commits, m.err
}

func TestRemoveUnchangedRepos_AllNew(t *testing.T) {
	dir := t.TempDir()

	repos := []*github.Repository{
		makeRepo("org1", "repo1"),
		makeRepo("org1", "repo2"),
	}

	commit1, commit2 := "abc123", "def456"
	lister := &mockLister{commits: []*string{&commit1, &commit2}}

	result, err := RemoveUnchangedRepos(lister, dir, repos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 repos, got %d", len(result))
	}
}

func TestRemoveUnchangedRepos_NoneChanged(t *testing.T) {
	dir := t.TempDir()

	repos := []*github.Repository{makeRepo("org1", "repo1")}
	commit := "abc123"
	lister := &mockLister{commits: []*string{&commit}}

	// First call — seeds repos.json
	_, err := RemoveUnchangedRepos(lister, dir, repos)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}

	// Second call with same commit — should return empty
	result, err := RemoveUnchangedRepos(lister, dir, repos)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 changed repos, got %d", len(result))
	}
}

func TestRemoveUnchangedRepos_SomeChanged(t *testing.T) {
	dir := t.TempDir()

	repos := []*github.Repository{
		makeRepo("org1", "repo1"),
		makeRepo("org1", "repo2"),
	}

	commit1, commit2 := "aaa", "bbb"
	lister := &mockLister{commits: []*string{&commit1, &commit2}}

	// Seed
	_, err := RemoveUnchangedRepos(lister, dir, repos)
	if err != nil {
		t.Fatal(err)
	}

	// Change only repo2's commit
	newCommit2 := "ccc"
	lister.commits = []*string{&commit1, &newCommit2}

	result, err := RemoveUnchangedRepos(lister, dir, repos)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 changed repo, got %d", len(result))
	}
	if result[0].GetFullName() != "org1/repo2" {
		t.Errorf("changed repo = %q, want %q", result[0].GetFullName(), "org1/repo2")
	}
}

func TestRemoveUnchangedRepos_CorruptJSON(t *testing.T) {
	dir := t.TempDir()

	// Write corrupt repos.json
	if err := os.WriteFile(filepath.Join(dir, "repos.json"), []byte("not json{{{"), 0644); err != nil {
		t.Fatal(err)
	}

	repos := []*github.Repository{makeRepo("org1", "repo1")}
	commit := "abc"
	lister := &mockLister{commits: []*string{&commit}}

	result, err := RemoveUnchangedRepos(lister, dir, repos)
	if err != nil {
		t.Fatalf("should recover from corrupt JSON, got error: %v", err)
	}

	// All repos should be treated as new
	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
}

func TestRemoveUnchangedRepos_WritesJSON(t *testing.T) {
	dir := t.TempDir()

	repos := []*github.Repository{makeRepo("org1", "repo1")}
	commit := "abc123"
	lister := &mockLister{commits: []*string{&commit}}

	_, err := RemoveUnchangedRepos(lister, dir, repos)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "repos.json"))
	if err != nil {
		t.Fatalf("repos.json not written: %v", err)
	}

	var j JsonRepos
	if err := json.Unmarshal(data, &j); err != nil {
		t.Fatalf("repos.json invalid JSON: %v", err)
	}

	if j.Repos["org1/repo1"] != "abc123" {
		t.Errorf("repos.json commit = %q, want %q", j.Repos["org1/repo1"], "abc123")
	}
}

func TestRemoveUnchangedRepos_NoReposJSON(t *testing.T) {
	dir := t.TempDir()

	// Don't create repos.json — should be auto-created
	repos := []*github.Repository{makeRepo("org1", "repo1")}
	commit := "abc"
	lister := &mockLister{commits: []*string{&commit}}

	result, err := RemoveUnchangedRepos(lister, dir, repos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}

	if !Exists(filepath.Join(dir, "repos.json")) {
		t.Error("repos.json was not created")
	}
}
