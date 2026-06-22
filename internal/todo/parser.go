package todo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"commitment-ledger/internal/model"
)

var (
	indexLinePattern   = regexp.MustCompile("^\\s*(?:\\[( |x|X)\\]\\s+)?([A-Z]+-[a-z]{5}|\\d{3})\\s*-\\s*(.+?)\\s*$")
	detailLinkPattern  = regexp.MustCompile("^(.*?)\\s+\\(`([^`]+)`\\)\\s*$")
	subtaskLinePattern = regexp.MustCompile("^\\s*[-*]\\s+\\[( |x|X)\\]\\s+((?:\\d+)(?:\\.\\d+)*)\\.?\\s+(.+?)\\s*$")
)

func Parse(repo string, branch string, commit string, todoPath string, seen time.Time, prior map[string]model.WorkItem) ([]model.WorkItem, error) {
	file, err := os.Open(todoPath)
	if err != nil {
		return nil, fmt.Errorf("open todo file %q: %w", todoPath, err)
	}
	defer file.Close()

	var items []model.WorkItem
	seenAt := seen.Format(time.RFC3339)
	repoRoot := filepath.Dir(filepath.Dir(todoPath))
	sourceFile := filepath.ToSlash(filepath.Base(filepath.Dir(todoPath)) + "/" + filepath.Base(todoPath))

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		match := indexLinePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		status := statusFromCheckbox(match[1])
		workID := match[2]
		title := match[3]
		detailFile := ""
		if link := detailLinkPattern.FindStringSubmatch(title); link != nil {
			title = strings.TrimSpace(link[1])
			detailFile = filepath.ToSlash(link[2])
		}

		item := model.WorkItem{
			Repo:       repo,
			Branch:     branch,
			Commit:     commit,
			WorkID:     workID,
			Title:      title,
			DetailFile: detailFile,
			Status:     status,
			SourceFile: sourceFile,
			FirstSeen:  firstSeen(prior, repo, branch, workID, seenAt),
			LastSeen:   seenAt,
		}
		items = append(items, item)

		if detailFile == "" {
			continue
		}

		subtasks, err := parseDetailFile(repo, branch, commit, repoRoot, detailFile, workID, seenAt, prior)
		if err != nil {
			return nil, err
		}
		items = append(items, subtasks...)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan todo file %q: %w", todoPath, err)
	}

	return items, nil
}

func parseDetailFile(repo string, branch string, commit string, repoRoot string, detailFile string, parentID string, seenAt string, prior map[string]model.WorkItem) ([]model.WorkItem, error) {
	path := filepath.Join(repoRoot, filepath.FromSlash(detailFile))
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open detail file %q: %w", detailFile, err)
	}
	defer file.Close()

	var subtasks []model.WorkItem
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		match := subtaskLinePattern.FindStringSubmatch(scanner.Text())
		if match == nil {
			continue
		}

		subID := parentID + "/" + match[2]
		subtasks = append(subtasks, model.WorkItem{
			Repo:       repo,
			Branch:     branch,
			Commit:     commit,
			WorkID:     subID,
			ParentWork: parentID,
			Title:      strings.TrimSpace(match[3]),
			DetailFile: detailFile,
			Status:     statusFromCheckbox(match[1]),
			SourceFile: detailFile,
			FirstSeen:  firstSeen(prior, repo, branch, subID, seenAt),
			LastSeen:   seenAt,
			IsSubtask:  true,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan detail file %q: %w", detailFile, err)
	}
	return subtasks, nil
}

func statusFromCheckbox(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "x") {
		return "complete"
	}
	return "open"
}

func firstSeen(prior map[string]model.WorkItem, repo string, branch string, workID string, fallback string) string {
	key := model.WorkTarget(repo, branch, workID)
	if prev, ok := prior[key]; ok && prev.FirstSeen != "" {
		return prev.FirstSeen
	}
	return fallback
}
