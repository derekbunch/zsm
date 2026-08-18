package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// SessionInfo is a single zellij session with its live/exited status.
type SessionInfo struct {
	Name   string
	Exited bool // true when listed as "EXITED - attach to resurrect"
}

// listSessions returns active and resurrectable zellij sessions.
// Format of `zellij ls -n` lines:
//
//	<name> [Created ... ago]            — live
//	<name> [Created ... ago] (current)  — the session we're attached to
//	<name> [Created ... ago] (EXITED - attach to resurrect)
func listSessions() ([]SessionInfo, error) {
	out, err := zellij("ls", "-n")
	if err != nil {
		return nil, nil
	}
	var sessions []SessionInfo
	for _, line := range splitLines(out) {
		idx := strings.Index(line, " ")
		if idx < 0 {
			continue
		}
		sessions = append(sessions, SessionInfo{
			Name:   line[:idx],
			Exited: strings.Contains(line, "EXITED"),
		})
	}
	return sessions, nil
}

// attachSession attaches to an existing zellij session.
func attachSession(name string) error {
	if os.Getenv("ZELLIJ") != "" {
		_, err := zellij("action", "switch-session", name)
		return err
	}
	return becomeZellij("attach", "-c", name)
}

// createSession creates a new zellij session in the given directory.
func createSession(name, dir, layout string) error {
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("chdir %s: %w", dir, err)
		}
	}

	if os.Getenv("ZELLIJ") != "" {
		args := []string{"action", "switch-session", name}
		if dir != "" {
			args = append(args, "--cwd", dir)
		}
		args = append(args, "--layout", orDefault(layout, "zjstatus"))
		if _, err := zellij(args...); err == nil {
			return nil
		} else if !isSessionNotFound(err) {
			return err
		}

		// Zellij 0.44 no longer creates missing sessions via switch-session.
		// Detach this client, then replace zsm with a normal Zellij create.
		if _, err := zellij("action", "detach"); err != nil {
			return err
		}
		return becomeNewSession(name, layout)
	}

	return becomeNewSession(name, layout)
}

func becomeNewSession(name, layout string) error {
	if layout != "" {
		return becomeZellijClean("-s", name, "-l", layout)
	}
	return becomeZellijClean("attach", "-c", name)
}

// deleteSession fully removes a zellij session (live or resurrectable).
// kill-session is a no-op on an exited session; then delete-session wipes the
// resurrect data.
func deleteSession(name string) error {
	_, _ = zellij("kill-session", name)
	_, err := zellij("delete-session", name)
	return err
}

// killSession stops a live session but leaves it resurrectable.
func killSession(name string) error {
	_, err := zellij("kill-session", name)
	return err
}

// renameSession renames a zellij session.
func renameSession(oldName, newName string) error {
	cmd := exec.Command("zellij", "action", "rename-session", newName)
	cmd.Env = append(os.Environ(), "ZELLIJ_SESSION_NAME="+oldName)
	return cmd.Run()
}

// dumpScreen returns the screen contents of the focused pane in a session (plain text).
func dumpScreen(sessionName string) (string, error) {
	return zellij("--session", sessionName, "action", "dump-screen")
}

// dumpScreenAnsi returns the screen contents with ANSI styling preserved.
func dumpScreenAnsi(sessionName string) (string, error) {
	return zellij("--session", sessionName, "action", "dump-screen", "--ansi")
}

// dumpLayout returns the KDL layout of a session (tabs, panes, floating panes).
// For live sessions it uses the zellij action; for exited sessions it reads the
// cached session-layout.kdl that zellij writes on kill.
func dumpLayout(sessionName string, exited bool) (string, error) {
	if exited {
		path := fmt.Sprintf("%s/org.Zellij-Contributors.Zellij/contract_version_1/session_info/%s/session-layout.kdl",
			os.Getenv("HOME")+"/Library/Caches", sessionName)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return zellij("--session", sessionName, "action", "dump-layout")
}

// zellij runs a zellij command as a subprocess and returns its output.
func zellij(args ...string) (string, error) {
	out, err := exec.Command("zellij", args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return "", fmt.Errorf("zellij %s: %w: %s", args[0], err, msg)
		}
		return "", fmt.Errorf("zellij %s: %w", args[0], err)
	}
	return string(out), nil
}

// becomeZellij replaces the current process with zellij (never returns on success).
func becomeZellij(args ...string) error {
	bin, err := exec.LookPath("zellij")
	if err != nil {
		return fmt.Errorf("zellij not found: %w", err)
	}
	return syscall.Exec(bin, append([]string{"zellij"}, args...), os.Environ())
}

func becomeZellijClean(args ...string) error {
	bin, err := exec.LookPath("zellij")
	if err != nil {
		return fmt.Errorf("zellij not found: %w", err)
	}
	return syscall.Exec(bin, append([]string{"zellij"}, args...), withoutZellijEnv(os.Environ()))
}

func withoutZellijEnv(env []string) []string {
	out := env[:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, "ZELLIJ=") || strings.HasPrefix(kv, "ZELLIJ_SESSION_NAME=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func isSessionNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found")
}

func orDefault(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
