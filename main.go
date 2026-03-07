package main

import (
	"context"
	"fmt"
	"time"

	"github.com/josiahbull/gubber/config"
	"github.com/josiahbull/gubber/download"
)

func main() {
	config, err := config.NewConfig()
	if err != nil {
		fmt.Printf("failed to load config due to error %v\n", err)
		panic(err)
	}

	ctx := context.Background()

	github := download.NewGitHubAPI(ctx, &config.Token)
	downloader := download.NewDownloader(ctx, &config.Token)

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
		repos, err = download.RemoveUnchangedRepos(github, config.Location, repos)
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
