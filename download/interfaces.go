package download

import "github.com/google/go-github/github"

// RepoLister abstracts GitHub API operations for listing and inspecting repos.
type RepoLister interface {
	GetOrgs() ([]*github.Organization, error)
	GetRepos() ([]*github.Repository, error)
	GetOrgRepos(org *github.Organization) ([]*github.Repository, error)
	RemoveEmptyRepos(repos []*github.Repository) ([]*github.Repository, error)
	GetLastCommits(repos []*github.Repository) ([]*string, error)
}

// RepoDownloader abstracts downloading repos from a remote source.
type RepoDownloader interface {
	DownloadRepos(repos []*github.Repository, location *string) error
}
