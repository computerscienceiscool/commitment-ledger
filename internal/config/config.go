package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type RepoSource struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	URL       string `json:"url"`
	LocalPath string `json:"local_path"`
	Branch    string `json:"branch"`
	TodoFile  string `json:"todo_file"`
	Enabled   bool   `json:"enabled"`
}

type ReposConfig struct {
	Repos []RepoSource `json:"repos"`
}

func Load(path string) (ReposConfig, error) {
	var cfg ReposConfig

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}

func (r RepoSource) ResolveTodoPath() string {
	return filepath.Join(r.LocalPath, r.TodoFile)
}
