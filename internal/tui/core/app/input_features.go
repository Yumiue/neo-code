package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"neo-code/internal/config"
	tuiinfra "neo-code/internal/tui/infra"
	tuiservices "neo-code/internal/tui/services"
)

const (
	workspaceCommandPrefix = "&"
	workspaceCommandUsage  = "& <command>"
	fileReferencePrefix    = "@"
	fileMenuTitle          = "Files"
	shellMenuTitle         = "Shell"
	maxWorkspaceFiles      = 4000
	maxFileSuggestions     = 6
)

type tokenSelector int

const (
	tokenSelectorFirst tokenSelector = iota
	tokenSelectorLast
)

var workspaceCommandExecutor = defaultWorkspaceCommandExecutor

func isWorkspaceCommandInput(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), workspaceCommandPrefix)
}

func extractWorkspaceCommand(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, workspaceCommandPrefix) {
		return "", fmt.Errorf("usage: %s", workspaceCommandUsage)
	}
	command := strings.TrimSpace(strings.TrimPrefix(trimmed, workspaceCommandPrefix))
	if command == "" {
		return "", fmt.Errorf("usage: %s", workspaceCommandUsage)
	}
	return command, nil
}

func runWorkspaceCommand(configManager *config.Manager, workdir string, raw string) tea.Cmd {
	return tuiservices.RunWorkspaceCommandCmd(
		func(ctx context.Context) (string, string, error) {
			return executeWorkspaceCommand(ctx, configManager, workdir, raw)
		},
		func(command string, output string, err error) tea.Msg {
			return workspaceCommandResultMsg{
				Command: command,
				Output:  output,
				Err:     err,
			}
		},
	)
}

func executeWorkspaceCommand(ctx context.Context, configManager *config.Manager, workdir string, raw string) (string, string, error) {
	command, err := extractWorkspaceCommand(raw)
	if err != nil {
		return "", "", err
	}

	cfg := configManager.Get()
	output, execErr := workspaceCommandExecutor(ctx, cfg, workdir, command)
	return command, output, execErr
}

func defaultWorkspaceCommandExecutor(ctx context.Context, cfg config.Config, workdir string, command string) (string, error) {
	return tuiinfra.DefaultWorkspaceCommandExecutor(ctx, cfg, workdir, command)
}

func shellArgs(shell string, command string) []string {
	return tuiinfra.ShellArgs(shell, command)
}

func powershellUTF8Command(command string) string {
	return tuiinfra.PowerShellUTF8Command(command)
}

func formatWorkspaceCommandResult(command string, output string, err error) string {
	header := "Command"
	if err != nil {
		header = "Command Failed"
	}

	body := strings.TrimSpace(output)
	if body == "" && err != nil {
		body = err.Error()
	}
	if body == "" {
		body = "(no output)"
	}

	body = strings.ReplaceAll(body, "```", "` ` `")
	return fmt.Sprintf("%s: & %s\n```text\n%s\n```", header, command, body)
}

func sanitizeWorkspaceOutput(raw []byte) string {
	return tuiinfra.SanitizeWorkspaceOutput(raw)
}

func decodeWorkspaceOutput(raw []byte) string {
	return tuiinfra.DecodeWorkspaceOutput(raw)
}

func (a *App) refreshFileCandidates() error {
	candidates, err := collectWorkspaceFiles(a.state.CurrentWorkdir, maxWorkspaceFiles)
	if err != nil {
		return err
	}
	a.fileCandidates = candidates
	if absolute := tuiservices.ResolveWorkspaceDirectory(a.state.CurrentWorkdir); absolute != "" {
		a.fileBrowser.CurrentDirectory = absolute
	}
	a.refreshCommandMenu()
	return nil
}

func collectWorkspaceFiles(root string, limit int) ([]string, error) {
	return tuiservices.CollectWorkspaceFiles(root, limit)
}

func (a App) resolveFileReferenceSuggestions(input string) (start int, end int, query string, suggestions []string, ok bool) {
	start, end, token, ok := currentReferenceToken(input)
	if !ok {
		return 0, 0, "", nil, false
	}

	query = strings.ToLower(strings.TrimPrefix(token, fileReferencePrefix))
	suggestions = collectFileSuggestionMatches(query, a.fileCandidates, maxFileSuggestions)
	return start, end, query, suggestions, true
}

func collectFileSuggestionMatches(query string, candidates []string, limit int) []string {
	return tuiservices.SuggestFileMatches(query, candidates, limit)
}

func tokenRange(input string, selector tokenSelector) (start int, end int, token string, ok bool) {
	if strings.TrimSpace(input) == "" {
		return 0, 0, "", false
	}

	switch selector {
	case tokenSelectorFirst:
		start = 0
		for start < len(input) {
			switch input[start] {
			case ' ', '\t', '\r', '\n':
				start++
			default:
				goto parse
			}
		}
		return 0, 0, "", false
	case tokenSelectorLast:
		end = len(input)
		start = strings.LastIndexAny(input, " \t\r\n")
		if start < 0 {
			start = 0
		} else {
			start++
		}
		if start >= end {
			return 0, 0, "", false
		}
		token = input[start:end]
		return start, end, token, true
	default:
		return 0, 0, "", false
	}

parse:
	end = start
	for end < len(input) {
		switch input[end] {
		case ' ', '\t', '\r', '\n':
			token = input[start:end]
			return start, end, token, true
		default:
			end++
		}
	}
	token = input[start:end]
	return start, end, token, true
}

func currentReferenceToken(input string) (start int, end int, token string, ok bool) {
	start, end, token, ok = tokenRange(input, tokenSelectorLast)
	if !ok {
		return 0, 0, "", false
	}
	if !strings.HasPrefix(token, fileReferencePrefix) {
		return 0, 0, "", false
	}
	return start, end, token, true
}

func (a *App) applyFileReference(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("file path is empty")
	}

	resolved := filepath.ToSlash(path)
	if workdir := strings.TrimSpace(a.state.CurrentWorkdir); workdir != "" {
		base, errBase := filepath.Abs(workdir)
		target, errTarget := filepath.Abs(path)
		if errBase == nil && errTarget == nil {
			if rel, errRel := filepath.Rel(base, target); errRel == nil && !strings.HasPrefix(rel, "..") {
				resolved = filepath.ToSlash(rel)
			} else {
				resolved = filepath.ToSlash(target)
			}
		}
	}
	resolved = strings.TrimPrefix(resolved, "./")
	reference := fileReferencePrefix + resolved

	current := a.input.Value()
	if start, end, _, ok := currentReferenceToken(current); ok {
		current = current[:start] + reference + current[end:]
	} else if strings.TrimSpace(current) == "" {
		current = reference
	} else {
		separator := " "
		if strings.HasSuffix(current, " ") || strings.HasSuffix(current, "\t") {
			separator = ""
		}
		current = current + separator + reference
	}

	a.input.SetValue(current)
	a.state.InputText = current
	a.normalizeComposerHeight()
	a.applyComponentLayout(false)
	a.refreshCommandMenu()
	a.state.StatusText = fmt.Sprintf("[System] Added file reference %s.", reference)
	return nil
}
