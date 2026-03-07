package download

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/google/go-github/github"
)

type JsonRepos struct {
	Repos map[string]string `json:"repos"`
}

func RemoveUnchangedRepos(lister RepoLister, location string, repos []*github.Repository) ([]*github.Repository, error) {
	commits, err := lister.GetLastCommits(repos)
	if err != nil {
		return nil, fmt.Errorf("failed to get last commits due to error %w", err)
	}

	file, err := os.Open(location + "/repos.json")

	if os.IsNotExist(err) {
		file, err = os.Create(location + "/repos.json")
		if err != nil {
			return nil, fmt.Errorf("failed to create repos.json due to error %w", err)
		}
		_, err := file.Write([]byte("{}"))
		if err != nil {
			return nil, fmt.Errorf("failed to write to repos.json due to error %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open repos.json due to error %w", err)
	}

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read repos.json due to error %w", err)
	}

	var jsonRepos JsonRepos
	err = json.Unmarshal(byteValue, &jsonRepos)
	if err != nil {
		fmt.Printf("failed to unmarshal repos.json due to error %v\n", err)
		jsonRepos = JsonRepos{
			Repos: make(map[string]string),
		}
	}

	newRepos := make([]*github.Repository, 0)
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

	err = os.WriteFile(location+"/repos.json", jsonReposBytes, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write to repos.json due to error %w", err)
	}

	return newRepos, nil
}
