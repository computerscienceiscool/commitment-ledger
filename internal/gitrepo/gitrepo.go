package gitrepo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type State struct {
	Branch string
	Commit string
}

func Observe(localPath string) (State, error) {
	if _, err := os.Stat(localPath); err != nil {
		return State{}, fmt.Errorf("stat repo %q: %w", localPath, err)
	}

	branch, err := runGit(localPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return State{}, fmt.Errorf("read branch for %q: %w", localPath, err)
	}
	commit, err := runGit(localPath, "rev-parse", "HEAD")
	if err != nil {
		return State{}, fmt.Errorf("read commit for %q: %w", localPath, err)
	}

	return State{
		Branch: branch,
		Commit: commit,
	}, nil
}

func runGit(localPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", localPath}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%v: %w (%s)", args, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
