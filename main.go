package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"

	"os/exec"
)

// get all orgs for the user, using pagination to get all orgs
func get_orgs(ctx context.Context, client *github.Client) ([]*github.Organization, error) {
	var orgs []*github.Organization = make([]*github.Organization, 0)
	var opts github.ListOptions = github.ListOptions{
		PerPage: 100,
	}
	for {
		new_orgs, resp, err := client.Organizations.List(ctx, "", &opts)
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

// get all repos for all provided orgs, using pagination to get all repos
func get_repos_from_orgs(ctx context.Context, client *github.Client, orgs []*github.Organization) ([]*github.Repository, error) {
	var repos []*github.Repository = make([]*github.Repository, 0)
	var opts github.RepositoryListByOrgOptions = github.RepositoryListByOrgOptions{
		Type: "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	for _, org := range orgs {
		for {
			new_repos, resp, err := client.Repositories.ListByOrg(ctx, org.GetLogin(), &opts)
			if err != nil {
				return nil, fmt.Errorf("failed to get repos for org: %w", err)
			}
			repos = append(repos, new_repos...)
			if resp.NextPage == 0 {
				break
			}
			opts.ListOptions.Page = resp.NextPage
		}
	}
	return repos, nil
}

func get_repos(ctx context.Context, client *github.Client) ([]*github.Repository, error) {
	// get all orgs
	fmt.Println("Getting orgs")
	orgs, err := get_orgs(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get orgs: %w", err)
	}
	fmt.Printf("Number of orgs found: %d\n", len(orgs))

	// get all repos
	fmt.Println("Getting repos")
	repos, err := get_repos_from_orgs(ctx, client, orgs)
	if err != nil {
		return nil, fmt.Errorf("failed to get repos: %w", err)
	}

	var opts github.RepositoryListOptions = github.RepositoryListOptions{
		Type: "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	for {
		new_repos, resp, err := client.Repositories.List(ctx, "", &opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get repos: %w", err)
		}
		repos = append(repos, new_repos...)
		if resp.NextPage == 0 {
			break
		}
		opts.ListOptions.Page = resp.NextPage
	}

	fmt.Println("Filtering empty repos")
	repos, err = filter_empty_repos(client, repos)
	if err != nil {
		return nil, fmt.Errorf("failed to filter empty repos: %w", err)
	}

	fmt.Printf("Number of repos found: %d\n", len(repos))
	return repos, nil
}

// filter repos that are empty
func filter_empty_repos(client *github.Client, repos []*github.Repository) ([]*github.Repository, error) {
	var filtered_repos []*github.Repository = make([]*github.Repository, 0)
	for _, repo := range repos {
		_, _, resp, err := client.Repositories.GetContents(context.Background(), repo.GetOwner().GetLogin(), repo.GetName(), "", nil)

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

// download a repo to the file system
func download_repo(ctx context.Context, client *github.Client, repo *github.Repository, token *string, download_location *string) error {
	if repo.GetFullName() == "" {
		return errors.New("repo name is empty")
	}

	// create the org folder if it doesn't exist
	fmt.Println("Creating folder:", *download_location+"/"+repo.GetFullName())
	org_folder := *download_location + "/" + repo.GetOwner().GetLogin()

	cmd := exec.CommandContext(ctx, "mkdir", "-p", org_folder)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create org folder due to error %w\nstdout + stderr: %s", err, output)
	}

	// download the repo
	fmt.Println("Downloading:", repo.GetFullName())
	cmd = exec.CommandContext(ctx, "git", "clone", "--mirror", "https://"+*token+"@github.com/"+repo.GetFullName()+".git", org_folder+"/"+repo.GetName()+".git")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to download repo due to error %w\nstdout + stderr: %s", err, output)
	}

	// bundle the repo
	fmt.Println("Bundling:", repo.GetFullName())
	cmd = exec.CommandContext(ctx, "git", "bundle", "create", repo.GetName()+".bundle", "--all")
	cmd.Dir = org_folder + "/" + repo.GetName() + ".git"

	// run command getting stdout and stderr
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to bundle repo due to error %w\nstdout + stderr: %s", err, output)
	}

	// move the bundle to the download location
	fmt.Println("Moving:", repo.GetFullName())
	cmd = exec.CommandContext(ctx, "mv", org_folder+"/"+repo.GetName()+".git/"+repo.GetName()+".bundle", org_folder)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to move bundle to download location due to error %w\nstdout + stderr: %s", err, output)
	}

	// delete the .git repo
	fmt.Println("Cleaning:", repo.GetFullName())
	cmd = exec.CommandContext(ctx, "rm", "-rf", org_folder+"/"+repo.GetName()+".git")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clean repo due to error %w\nstdout + stderr: %s", err, output)
	}

	return nil
}

// initiate download of the repos to the file system
func download_repos(ctx context.Context, client *github.Client, repos []*github.Repository, token *string, download_location *string) error {
	for _, repo := range repos {
		err := download_repo(ctx, client, repo, token, download_location)
		if err != nil {
			return fmt.Errorf("failed to download repo %s due to error %w", repo.GetFullName(), err)
		}
	}
	return nil
}

func main() {
	// parse the toml/yaml/env file
	token := os.Getenv("GITHUB_TOKEN")
	repo_location := os.Getenv("LOCATION")

	// login to github
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// collect repos
	repos, err := get_repos(ctx, client)
	if err != nil {
		fmt.Println("failed to get repos due to error:", err)
		return
	}

	// download all repos to a temporary location
	temp_location, err := ioutil.TempDir("", "repository")
	if err != nil {
		fmt.Println("failed to create temporary location due to error:", err)
		return
	}
	defer os.RemoveAll(temp_location)
	err = download_repos(ctx, client, repos, &token, &temp_location)
	if err != nil {
		fmt.Println(err)
		return
	}

	// for every folder called "backT-x" in the repo location, increment x in it's name by one and move it
	items, err := ioutil.ReadDir(repo_location)
	if err != nil {
		fmt.Println("failed to read repo location due to error:", err)
		return
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
					fmt.Println("failed to convert folder name to int due to error:", err)
					return
				}

				new_name := "backT-" + strconv.Itoa(file_number+1)
				err = os.Rename(repo_location+"/"+item.Name(), repo_location+"/"+new_name)
				if err != nil {
					fmt.Println("failed to rename folder due to error:", err)
					return
				}
			}
		}
	}

	// move the temporary location to the repo location, naming it "backT-0"
	err = os.Rename(temp_location, repo_location+"/backT-0")
	if err != nil {
		fmt.Println("failed to move temporary location to repo location due to error:", err)
		return
	}

	fmt.Println("done")
}
