package main

import (
	"context"
	"fmt"
	"time"

	"github.com/josiahbull/gubber/config"
	"github.com/josiahbull/gubber/downloader"
)

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
			time.Sleep(time.Duration(config.Interval))
		}
		first = false

		repos, err := github.GetRepos()
		if err != nil {
			fmt.Printf("failed to get repos due to error %v\n", err)
			continue
		}

		err = downloader.MigrateRepos(repos, &config.Location)
		if err != nil {
			fmt.Printf("failed to migrate repos due to error %v\n", err)
			continue
		}

		fmt.Println("Done")
	}
}
