package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	githubSource "github.com/google/go-github/github"

	"github.com/josiahbull/gubber/config"
	"github.com/josiahbull/gubber/downloader"
)

type JsonRepos struct {
	Repos map[string]string `json:"repos"`
}

func removeUnchangedRepos(github downloader.GitHubAPI, config config.Config, repos []*githubSource.Repository) ([]*githubSource.Repository, error) {
	// load the last commit for each repo
	commits, err := github.GetLastCommits(repos)
	if err != nil {
		return nil, fmt.Errorf("failed to get last commits due to error %w", err)
	}

	// load the json file from disk, with all the repos that we have downloaded and their latest commit
	file, err := os.Open(config.Location + "/repos.json")

	// create repos.json if it doesn't exist
	if os.IsNotExist(err) {
		file, err = os.Create(config.Location + "/repos.json")
		if err != nil {
			return nil, fmt.Errorf("failed to create repos.json due to error %w", err)
		}
		// write {} to the file
		_, err := file.Write([]byte("{}"))
		if err != nil {
			return nil, fmt.Errorf("failed to write to repos.json due to error %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open repos.json due to error %w", err)
	}

	byteValue, err := ioutil.ReadAll(file)

	if err != nil {
		return nil, fmt.Errorf("failed to read repos.json due to error %w", err)
	}

	var jsonRepos JsonRepos
	err = json.Unmarshal(byteValue, &jsonRepos)
	if err != nil {
		fmt.Printf("failed to unmarshal repos.json due to error %v\n", err)
		// set jsonRepos to an empty struct
		jsonRepos = JsonRepos{
			Repos: make(map[string]string),
		}
	}

	// if a repo with the same commit is already in the map, remove it from the list of repos to download
	newRepos := make([]*githubSource.Repository, 0)
	for i, repo := range repos {
		if commit, ok := jsonRepos.Repos[repo.GetFullName()]; ok {
			if commit != *commits[i] {
				newRepos = append(newRepos, repo)
			}
		} else {
			newRepos = append(newRepos, repo)
		}
		jsonRepos.Repos[repo.GetFullName()] = *commits[i]
	}

	jsonReposBytes, err := json.Marshal(jsonRepos)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jsonRepos due to error %w", err)
	}

	err = ioutil.WriteFile(config.Location+"/repos.json", jsonReposBytes, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write to repos.json due to error %w", err)
	}

	return newRepos, nil
}

func main() {
	config := config.NewConfig()
	err := config.Validate()
	if err != nil {
		fmt.Printf("config error: %s\n", err)
		panic(err)
	}

	ctx := context.Background()

	github := downloader.NewGitHubAPI(ctx, &config.Token)
	downloader := downloader.NewDownloader(ctx, &config.Token)

	first := true
	for {
		if !first {
			fmt.Printf("Sleeping for %d seconds\n", config.Interval)
			time.Sleep(time.Duration(config.Interval) * time.Second)
		}
		first = false

		fmt.Println("Loading all repositories")

		orgs, err := github.GetOrgs()
		if err != nil {
			fmt.Printf("failed to get orgs due to error %v\n", err)
			continue
		}

		repos, err := github.GetRepos()
		if err != nil {
			fmt.Printf("failed to get repos due to error %v\n", err)
			continue
		}

		// load all the repos from the orgs
		for _, org := range orgs {
			fmt.Printf("Loading repos for org %s\n", org.GetLogin())
			orgRepos, err := github.GetOrgRepos(org)
			if err != nil {
				fmt.Printf("failed to get org repos due to error %v for org %s\n", err, org.GetLogin())
				continue
			}

			// avoid duplicates by only adding repos that are not already in the list
			for _, orgRepo := range orgRepos {
				found := false
				for _, repo := range repos {
					if repo.GetFullName() == orgRepo.GetFullName() {
						found = true
						break
					}
				}
				if !found {
					repos = append(repos, orgRepo)
				}
			}
		}

		fmt.Printf("Found %d repositories\n", len(repos))

		fmt.Println("Removing empty repositories")

		repos, err = github.RemoveEmptyRepos(repos)
		if err != nil {
			fmt.Printf("failed to remove empty repos due to error %v\n", err)
			continue
		}

		fmt.Printf("Found %d repositories\n", len(repos))

		fmt.Println("Removing unchanged repositories")
		repos, err = removeUnchangedRepos(*github, *config, repos)
		if err != nil {
			fmt.Printf("failed to remove unchanged repos due to error %v\n", err)
			continue
		}

		// download all the repos that we have not downloaded yet
		if len(repos) == 0 {
			fmt.Println("No repos to download")
			continue
		}

		fmt.Printf("Downloading %d repos\n", len(repos))

		fmt.Printf("Downloading all %d repositories, and migrating old ones\n", len(repos))

		err = downloader.MigrateRepos(repos, &config.Location, config.Backups, &config.TempLocation)
		if err != nil {
			fmt.Printf("failed to migrate repos due to error %v\n", err)
			continue
		}

		fmt.Println("Done")
	}
}
