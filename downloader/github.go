package downloader

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type GitHubAPI struct {
	ctx    context.Context
	client *github.Client
}

func NewGitHubAPI(ctx context.Context, token *string) *GitHubAPI {
	// login to github
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &GitHubAPI{
		ctx:    ctx,
		client: client,
	}
}

// GetOrgs returns a list of organizations that the user can access
func (g *GitHubAPI) GetOrgs() ([]*github.Organization, error) {
	var orgs []*github.Organization = make([]*github.Organization, 0)
	var opts github.ListOptions = github.ListOptions{
		PerPage: 100,
	}
	for {
		new_orgs, resp, err := g.client.Organizations.List(g.ctx, "", &opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get orgs: %w", err)
		}
		orgs = append(orgs, new_orgs...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return orgs, nil
}

// GetRepos returns a list of repositories that the user can access, excluding orgs
func (g *GitHubAPI) GetRepos() ([]*github.Repository, error) {
	var opts github.RepositoryListOptions = github.RepositoryListOptions{
		Type: "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	repos := make([]*github.Repository, 0)
	for {
		new_repos, resp, err := g.client.Repositories.List(g.ctx, "", &opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get repos: %w", err)
		}
		repos = append(repos, new_repos...)
		if resp.NextPage == 0 {
			break
		}
		opts.ListOptions.Page = resp.NextPage
	}
	return repos, nil
}

// GetOrgRepos returns a list of repositories that the user can access from the provided org
func (g *GitHubAPI) GetOrgRepos(org *github.Organization) ([]*github.Repository, error) {
	opts := github.RepositoryListByOrgOptions{
		Type: "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	repos := make([]*github.Repository, 0)
	for {
		new_repos, resp, err := g.client.Repositories.ListByOrg(g.ctx, org.GetLogin(), &opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get repos for org: %w", err)
		}
		repos = append(repos, new_repos...)
		if resp.NextPage == 0 {
			break
		}
		opts.ListOptions.Page = resp.NextPage
	}
	return repos, nil
}

// RemoveEmptyRepos removes repositories that have no files available, returning the list otherwise unchanged
func (g *GitHubAPI) RemoveEmptyRepos(repos []*github.Repository) ([]*github.Repository, error) {
	var filtered_repos []*github.Repository = make([]*github.Repository, 0)
	for _, repo := range repos {
		_, _, resp, err := g.client.Repositories.GetContents(context.Background(), repo.GetOwner().GetLogin(), repo.GetName(), "", nil)

		// will return 404 error if the repo is empty
		if err != nil {
			if resp.StatusCode == 404 {
				continue
			} else {
				return nil, fmt.Errorf("failed to get contents of repo: %w", err)
			}
		}

		filtered_repos = append(filtered_repos, repo)
	}
	return filtered_repos, nil
}

// GetAllAccessibleRepos returns a list of all repositories that this account has access to
func (g *GitHubAPI) GetAllAccessibleRepos() ([]*github.Repository, error) {
	orgs, err := g.GetOrgs()
	if err != nil {
		return nil, fmt.Errorf("failed to get orgs: %w", err)
	}
	repos := make([]*github.Repository, 0)
	for _, org := range orgs {
		org_repos, err := g.GetOrgRepos(org)
		if err != nil {
			return nil, fmt.Errorf("failed to get org repos: %w", err)
		}
		repos = append(repos, org_repos...)
	}
	return repos, nil
}
