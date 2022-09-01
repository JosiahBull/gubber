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
	Repos   []string `json:"repos"`
	Commits []string `json:"commits"`
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

		repos, err := github.GetAllAccessibleRepos()
		if err != nil {
			fmt.Printf("failed to get repos due to error %v\n", err)
			continue
		}

		fmt.Println("Removing empty repositories")

		repos, err = github.RemoveEmptyRepos(repos)
		if err != nil {
			fmt.Printf("failed to remove empty repos due to error %v\n", err)
			continue
		}

		fmt.Printf("Found %d repositories\n", len(repos))

		// load the last commit for each repo
		commits, err := github.GetLastCommits(repos)
		if err != nil {
			fmt.Printf("failed to get last commits due to error %v\n", err)
			continue
		}

		// load the json file from disk, with all the repos that we have downloaded and their latest commit
		file, err := os.Open(config.Location + "repos.json")

		// create repos.json if it doesn't exist
		if os.IsNotExist(err) {
			file, err = os.Create(config.Location + "repos.json")
			if err != nil {
				fmt.Printf("failed to create repos.json due to error %v\n", err)
				continue
			}
			// write {} to the file
			_, err := file.Write([]byte("{}"))
			if err != nil {
				fmt.Printf("failed to write to repos.json due to error %v\n", err)
				continue
			}
		}

		if err != nil {
			fmt.Printf("failed to open repos.json due to error %v\n", err)
			continue
		}

		byteValue, err := ioutil.ReadAll(file)

		if err != nil {
			fmt.Printf("failed to read repos.json due to error %v\n", err)
			continue
		}

		var jsonRepos JsonRepos
		err = json.Unmarshal(byteValue, &jsonRepos)
		if err != nil {
			fmt.Printf("failed to unmarshal repos.json due to error %v\n", err)
			continue
		}

		// convert jsonRepos to a map of repos to commits
		jsonReposMap := make(map[string]string)
		for i, repo := range jsonRepos.Repos {
			jsonReposMap[repo] = jsonRepos.Commits[i]
		}

		// if a repo with the same commit is already in the map, remove it from the list of repos to download
		newRepos := make([]*githubSource.Repository, 0)
		newCommits := make([]*string, 0)
		for i, repo := range repos {
			if commit, ok := jsonReposMap[repo.GetFullName()]; ok {
				if commit != *commits[i] {
					newRepos = append(newRepos, repo)
					newCommits = append(newCommits, commits[i])
				}
			} else {
				newRepos = append(newRepos, repo)
				newCommits = append(newCommits, commits[i])
			}
		}

		fmt.Printf("Found %d new repos\n", len(newRepos))

		repos = newRepos
		commits = newCommits

		// convert repos to a map of repos to commits, and save it to disk
		reposMap := make(map[string]string)
		for i, repo := range repos {
			reposMap[repo.GetFullName()] = *commits[i]
		}

		jsonRepos.Repos = make([]string, 0, len(reposMap))
		jsonRepos.Commits = make([]string, 0, len(reposMap))
		for repo, commit := range reposMap {
			jsonRepos.Repos = append(jsonRepos.Repos, repo)
			jsonRepos.Commits = append(jsonRepos.Commits, commit)
		}

		jsonReposBytes, err := json.Marshal(jsonRepos)
		if err != nil {
			fmt.Printf("failed to marshal repos.json to disk due to error %v\n", err)
			continue
		}

		err = ioutil.WriteFile("repos.json", jsonReposBytes, 0644)
		if err != nil {
			fmt.Printf("failed to write repos.json due to error %v\n", err)
			continue
		}

		// download all the repos that we have not downloaded yet
		if len(repos) == 0 {
			fmt.Println("No repos to download")
			continue
		}

		fmt.Printf("Downloading %d repos\n", len(repos))

		fmt.Printf("Downloading all %d repositories, and migrating old ones\n", len(repos))

		err = downloader.MigrateRepos(repos, &config.Location, config.Backups)
		if err != nil {
			fmt.Printf("failed to migrate repos due to error %v\n", err)
			continue
		}

		fmt.Println("Done")
	}
}
