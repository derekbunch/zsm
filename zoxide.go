package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// ZoxideEntry is a directory with its zoxide frecency score.
type ZoxideEntry struct {
	Dir   string
	Score float64
}

// incrementScore bumps a path's zoxide score.
func incrementScore(path string) error {
	return exec.Command("zoxide", "edit", "increment", path).Run()
}

// decrementScore lowers a path's zoxide score.
func decrementScore(path string) error {
	return exec.Command("zoxide", "edit", "decrement", path).Run()
}

// deleteZoxide removes a path from the zoxide database entirely.
func deleteZoxide(path string) error {
	return exec.Command("zoxide", "edit", "delete", path).Run()
}

// listDirectories returns ranked directories from zoxide with their scores.
// Format of `zoxide query -ls` is "<score> <path>" with leading-padded score.
func listDirectories() ([]ZoxideEntry, error) {
	out, err := exec.Command("zoxide", "query", "-ls").Output()
	if err != nil {
		return nil, nil
	}

	var entries []ZoxideEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split on first space: score then path
		idx := strings.Index(line, " ")
		if idx < 0 {
			continue
		}
		score, err := strconv.ParseFloat(line[:idx], 64)
		if err != nil {
			continue
		}
		dir := strings.TrimSpace(line[idx+1:])
		if dir != "" {
			entries = append(entries, ZoxideEntry{Dir: dir, Score: score})
		}
	}
	return entries, nil
}
