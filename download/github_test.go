package download

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/github"
)

func TestNewGitHubAPI(t *testing.T) {
	token := "test-token"
	ctx := context.Background()
	api := NewGitHubAPI(ctx, &token)
	if api == nil {
		t.Fatal("NewGitHubAPI returned nil")
	}
	if api.client == nil {
		t.Fatal("client is nil")
	}
}

func TestGetOrgs_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/orgs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*github.Organization{})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	orgs, err := api.GetOrgs()
	if err != nil {
		t.Fatalf("GetOrgs() error: %v", err)
	}
	if len(orgs) != 0 {
		t.Errorf("expected 0 orgs, got %d", len(orgs))
	}
}

func TestGetRepos_SinglePage(t *testing.T) {
	name := "test-repo"
	fullName := "user/test-repo"

	mux := http.NewServeMux()
	mux.HandleFunc("/user/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*github.Repository{
			{Name: &name, FullName: &fullName},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	repos, err := api.GetRepos()
	if err != nil {
		t.Fatalf("GetRepos() error: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].GetFullName() != "user/test-repo" {
		t.Errorf("repo name = %q, want %q", repos[0].GetFullName(), "user/test-repo")
	}
}

func TestGetLastCommit_HashesEvents(t *testing.T) {
	owner := "org"
	name := "repo"
	fullName := "org/repo"
	login := "org"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := "12345"
		eventType := "PushEvent"
		json.NewEncoder(w).Encode([]*github.Event{
			{ID: &id, Type: &eventType},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	repo := &github.Repository{
		Name:     &name,
		FullName: &fullName,
		Owner:    &github.User{Login: &login},
	}

	commit, err := api.GetLastCommit(repo)
	if err != nil {
		t.Fatalf("GetLastCommit() error: %v", err)
	}
	if commit == nil || *commit == "" {
		t.Error("expected non-empty commit hash")
	}

	// Same events should produce same hash (deterministic)
	commit2, err := api.GetLastCommit(repo)
	if err != nil {
		t.Fatal(err)
	}
	if *commit != *commit2 {
		t.Errorf("non-deterministic hash: %q != %q", *commit, *commit2)
	}

	// Verify it's a hex-encoded sha256 (64 chars)
	if len(*commit) != 64 {
		t.Errorf("hash length = %d, want 64", len(*commit))
	}

	_ = owner
}

func TestGetOrgRepos_SinglePage(t *testing.T) {
	orgLogin := "myorg"
	repoName := "org-repo"
	repoFullName := "myorg/org-repo"

	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/myorg/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*github.Repository{
			{Name: &repoName, FullName: &repoFullName},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	org := &github.Organization{Login: &orgLogin}

	repos, err := api.GetOrgRepos(org)
	if err != nil {
		t.Fatalf("GetOrgRepos() error: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].GetFullName() != "myorg/org-repo" {
		t.Errorf("repo = %q, want %q", repos[0].GetFullName(), "myorg/org-repo")
	}
}

func TestGetOrgs_Paginated(t *testing.T) {
	login1 := "org1"
	login2 := "org2"
	login3 := "org3"

	mux := http.NewServeMux()
	var server *httptest.Server

	mux.HandleFunc("/user/orgs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("page")
		switch page {
		case "", "1":
			w.Header().Set("Link", fmt.Sprintf(`<%s/user/orgs?page=2>; rel="next"`, server.URL))
			json.NewEncoder(w).Encode([]*github.Organization{
				{Login: &login1},
				{Login: &login2},
			})
		case "2":
			json.NewEncoder(w).Encode([]*github.Organization{
				{Login: &login3},
			})
		}
	})

	server = httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	orgs, err := api.GetOrgs()
	if err != nil {
		t.Fatalf("GetOrgs() error: %v", err)
	}
	if len(orgs) != 3 {
		t.Fatalf("expected 3 orgs, got %d", len(orgs))
	}
	if orgs[0].GetLogin() != "org1" {
		t.Errorf("orgs[0] login = %q, want %q", orgs[0].GetLogin(), "org1")
	}
	if orgs[1].GetLogin() != "org2" {
		t.Errorf("orgs[1] login = %q, want %q", orgs[1].GetLogin(), "org2")
	}
	if orgs[2].GetLogin() != "org3" {
		t.Errorf("orgs[2] login = %q, want %q", orgs[2].GetLogin(), "org3")
	}
}

func TestGetRepos_Paginated(t *testing.T) {
	name1 := "repo1"
	fullName1 := "user/repo1"
	name2 := "repo2"
	fullName2 := "user/repo2"
	name3 := "repo3"
	fullName3 := "user/repo3"

	mux := http.NewServeMux()
	var server *httptest.Server

	mux.HandleFunc("/user/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("page")
		switch page {
		case "", "1":
			w.Header().Set("Link", fmt.Sprintf(`<%s/user/repos?page=2>; rel="next"`, server.URL))
			json.NewEncoder(w).Encode([]*github.Repository{
				{Name: &name1, FullName: &fullName1},
				{Name: &name2, FullName: &fullName2},
			})
		case "2":
			json.NewEncoder(w).Encode([]*github.Repository{
				{Name: &name3, FullName: &fullName3},
			})
		}
	})

	server = httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	repos, err := api.GetRepos()
	if err != nil {
		t.Fatalf("GetRepos() error: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
	if repos[0].GetFullName() != "user/repo1" {
		t.Errorf("repos[0] = %q, want %q", repos[0].GetFullName(), "user/repo1")
	}
	if repos[1].GetFullName() != "user/repo2" {
		t.Errorf("repos[1] = %q, want %q", repos[1].GetFullName(), "user/repo2")
	}
	if repos[2].GetFullName() != "user/repo3" {
		t.Errorf("repos[2] = %q, want %q", repos[2].GetFullName(), "user/repo3")
	}
}

func TestRemoveEmptyRepos_FiltersEmpty(t *testing.T) {
	login := "org"
	hasContentName := "has-content"
	hasContentFullName := "org/has-content"
	emptyName := "empty-repo"
	emptyFullName := "org/empty-repo"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/has-content/contents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"name":"README.md","type":"file"}]`))
	})
	mux.HandleFunc("/repos/org/empty-repo/contents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"Not Found"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	repos := []*github.Repository{
		{Name: &hasContentName, FullName: &hasContentFullName, Owner: &github.User{Login: &login}},
		{Name: &emptyName, FullName: &emptyFullName, Owner: &github.User{Login: &login}},
	}

	result, err := api.RemoveEmptyRepos(repos)
	if err != nil {
		t.Fatalf("RemoveEmptyRepos() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(result))
	}
	if result[0].GetName() != "has-content" {
		t.Errorf("expected repo name %q, got %q", "has-content", result[0].GetName())
	}
}

func TestRemoveEmptyRepos_APIError(t *testing.T) {
	login := "org"
	name := "error-repo"
	fullName := "org/error-repo"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/error-repo/contents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte(`{"message":"Internal Server Error"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	repos := []*github.Repository{
		{Name: &name, FullName: &fullName, Owner: &github.User{Login: &login}},
	}

	_, err := api.RemoveEmptyRepos(repos)
	if err == nil {
		t.Fatal("expected error from RemoveEmptyRepos(), got nil")
	}
}

func TestRemoveEmptyRepos_AllEmpty(t *testing.T) {
	login := "org"
	name1 := "empty-one"
	fullName1 := "org/empty-one"
	name2 := "empty-two"
	fullName2 := "org/empty-two"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/empty-one/contents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"Not Found"}`))
	})
	mux.HandleFunc("/repos/org/empty-two/contents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"Not Found"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	repos := []*github.Repository{
		{Name: &name1, FullName: &fullName1, Owner: &github.User{Login: &login}},
		{Name: &name2, FullName: &fullName2, Owner: &github.User{Login: &login}},
	}

	result, err := api.RemoveEmptyRepos(repos)
	if err != nil {
		t.Fatalf("RemoveEmptyRepos() error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 repos, got %d", len(result))
	}
}

func TestRemoveEmptyRepos_NoneEmpty(t *testing.T) {
	login := "org"
	name1 := "repo-one"
	fullName1 := "org/repo-one"
	name2 := "repo-two"
	fullName2 := "org/repo-two"

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo-one/contents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"name":"README.md","type":"file"}]`))
	})
	mux.HandleFunc("/repos/org/repo-two/contents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"name":"README.md","type":"file"}]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	api := &GitHubAPI{ctx: context.Background(), client: client}
	repos := []*github.Repository{
		{Name: &name1, FullName: &fullName1, Owner: &github.User{Login: &login}},
		{Name: &name2, FullName: &fullName2, Owner: &github.User{Login: &login}},
	}

	result, err := api.RemoveEmptyRepos(repos)
	if err != nil {
		t.Fatalf("RemoveEmptyRepos() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(result))
	}
	if result[0].GetName() != "repo-one" {
		t.Errorf("expected first repo %q, got %q", "repo-one", result[0].GetName())
	}
	if result[1].GetName() != "repo-two" {
		t.Errorf("expected second repo %q, got %q", "repo-two", result[1].GetName())
	}
}
