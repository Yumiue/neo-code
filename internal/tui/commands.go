package tui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"neo-code/internal/config"
	"neo-code/internal/provider"
)

const (
	slashPrefix           = "/"
	slashCommandHelp      = "/help"
	slashCommandExit      = "/exit"
	slashCommandClear     = "/clear"
	slashCommandStatus    = "/status"
	slashCommandRun       = "/run"
	slashCommandGit       = "/git"
	slashCommandFile      = "/file"
	slashCommandPlan      = "/plan"
	slashCommandUndo      = "/undo"
	slashCommandProvider  = "/provider"
	slashCommandSetting   = "/setting"
	slashCommandSet       = "/set"
	slashCommandModelPick = "/model"

	slashUsageHelp     = "/help"
	slashUsageExit     = "/exit"
	slashUsageClear    = "/clear"
	slashUsageStatus   = "/status"
	slashUsageRun      = "/run <command>"
	slashUsageGit      = "/git <args>"
	slashUsageFile     = "/file <read|write|list> ..."
	slashUsagePlan     = "/plan"
	slashUsageUndo     = "/undo"
	slashUsageProvider = "/provider"
	slashUsageSetting  = "/setting [provider|model|workdir]"
	slashUsageSetURL   = "/set url <url>"
	slashUsageSetKey   = "/set key <key>"
	slashUsageModel    = "/model"

	commandMenuTitle       = "Commands"
	providerPickerTitle    = "Select Provider"
	providerPickerSubtitle = "Up/Down choose, Enter confirm, Esc cancel"
	modelPickerTitle       = "Select Model"
	modelPickerSubtitle    = "Up/Down choose, Enter confirm, Esc cancel"

	sidebarTitle      = "Sessions"
	sidebarFilterHint = "Type / to search"
	sidebarOpenHint   = "Enter to open"

	draftSessionTitle     = "Draft"
	emptyConversationText = "No conversation yet.\nAsk NeoCode to inspect or change code, or type /help to browse local commands."
	emptyMessageText      = "(empty)"

	statusReady           = "Ready"
	statusRuntimeClosed   = "Runtime closed"
	statusThinking        = "Thinking"
	statusCanceling       = "Canceling"
	statusCanceled        = "Canceled"
	statusRunningTool     = "Running tool"
	statusToolFinished    = "Tool finished"
	statusToolError       = "Tool error"
	statusError           = "Error"
	statusDraft           = "New draft"
	statusRunning         = "Running"
	statusApplyingCommand = "Applying local command"
	statusChooseProvider  = "Choose a provider"
	statusChooseModel     = "Choose a model"

	focusLabelSessions   = "Sessions"
	focusLabelTranscript = "Transcript"
	focusLabelComposer   = "Composer"

	messageTagUser  = "[ YOU ]"
	messageTagAgent = "[ NEO ]"
	messageTagTool  = "[ TOOL ]"

	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"
	roleEvent     = "event"
	roleError     = "error"
	roleSystem    = "system"
)

type slashCommand struct {
	Usage       string
	Description string
}

type commandSuggestion struct {
	Command slashCommand
	Match   bool
}

var builtinSlashCommands = []slashCommand{
	{Usage: slashUsageHelp, Description: "Show slash command help"},
	{Usage: slashUsageClear, Description: "Clear the current draft transcript"},
	{Usage: slashUsageStatus, Description: "Show workspace and Git status"},
	{Usage: slashUsageRun, Description: "Run a shell command inside the workspace"},
	{Usage: slashUsageGit, Description: "Run a Git command in the workspace"},
	{Usage: slashUsageFile, Description: "Read, write, or list workspace files"},
	{Usage: slashUsagePlan, Description: "Generate a local task plan"},
	{Usage: slashUsageUndo, Description: "Undo the last local transcript entry"},
	{Usage: slashUsageProvider, Description: "Open the interactive provider picker"},
	{Usage: slashUsageSetting, Description: "Show or change local settings"},
	{Usage: slashUsageSetURL, Description: "Set the API Base URL"},
	{Usage: slashUsageSetKey, Description: "Update the API key"},
	{Usage: slashUsageModel, Description: "Open the interactive model picker"},
	{Usage: slashUsageExit, Description: "Exit NeoCode"},
}

var shellCommandExecutor = defaultShellCommandExecutor

func newSelectionPicker(items []list.Item) list.Model {
	delegate := list.NewDefaultDelegate()
	picker := list.New(items, delegate, 0, 0)
	picker.Title = ""
	picker.SetShowHelp(false)
	picker.SetShowStatusBar(false)
	picker.SetFilteringEnabled(false)
	picker.DisableQuitKeybindings()
	return picker
}

func newProviderPicker(items []provider.ProviderCatalogItem) list.Model {
	listItems := make([]list.Item, 0, len(items))
	for _, item := range items {
		listItems = append(listItems, providerItem{
			id:          item.ID,
			name:        item.Name,
			description: item.Description,
		})
	}
	return newSelectionPicker(listItems)
}

func newModelPicker(models []provider.ModelDescriptor) list.Model {
	items := make([]list.Item, 0, len(models))
	for _, option := range models {
		items = append(items, modelItem{
			id:          option.ID,
			name:        option.Name,
			description: option.Description,
		})
	}
	return newSelectionPicker(items)
}

func replacePickerItems(current list.Model, next list.Model) list.Model {
	next.SetSize(current.Width(), current.Height())
	return next
}

func (a *App) refreshProviderPicker() error {
	items, err := a.providerSvc.ListProviders(context.Background())
	if err != nil {
		return err
	}

	a.providerPicker = replacePickerItems(a.providerPicker, newProviderPicker(items))
	a.selectCurrentProvider(a.state.CurrentProvider)
	return nil
}

func (a *App) refreshModelPicker() error {
	models, err := a.providerSvc.ListModels(context.Background())
	if err != nil {
		return err
	}

	a.modelPicker = replacePickerItems(a.modelPicker, newModelPicker(models))
	a.selectCurrentModel(a.state.CurrentModel)
	return nil
}

func (a *App) openProviderPicker() {
	a.state.ActivePicker = pickerProvider
	a.state.StatusText = statusChooseProvider
	a.input.Blur()
	a.selectCurrentProvider(a.state.CurrentProvider)
}

func (a *App) openModelPicker() {
	a.state.ActivePicker = pickerModel
	a.state.StatusText = statusChooseModel
	a.input.Blur()
	a.selectCurrentModel(a.state.CurrentModel)
}

func (a *App) closePicker() {
	a.state.ActivePicker = pickerNone
	a.focus = panelInput
	a.applyFocus()
}

func (a *App) selectCurrentProvider(providerID string) {
	items := a.providerPicker.Items()
	for idx, item := range items {
		candidate, ok := item.(providerItem)
		if ok && strings.EqualFold(candidate.id, providerID) {
			a.providerPicker.Select(idx)
			return
		}
	}
	if len(items) > 0 {
		a.providerPicker.Select(0)
	}
}

func (a *App) selectCurrentModel(modelID string) {
	items := a.modelPicker.Items()
	for idx, item := range items {
		candidate, ok := item.(modelItem)
		if ok && strings.EqualFold(candidate.id, modelID) {
			a.modelPicker.Select(idx)
			return
		}
	}
	if len(items) > 0 {
		a.modelPicker.Select(0)
	}
}

func (a App) matchingSlashCommands(input string) []commandSuggestion {
	if !strings.HasPrefix(input, slashPrefix) {
		return nil
	}

	query := strings.ToLower(strings.TrimSpace(input))
	out := make([]commandSuggestion, 0, len(builtinSlashCommands))
	for _, command := range builtinSlashCommands {
		normalized := strings.ToLower(command.Usage)
		match := query == slashPrefix || strings.HasPrefix(normalized, query)
		if query == slashPrefix || match || strings.Contains(normalized, query) {
			out = append(out, commandSuggestion{Command: command, Match: match})
		}
	}
	return out
}

func runProviderSelection(providerSvc ProviderController, providerName string) tea.Cmd {
	return func() tea.Msg {
		selection, err := providerSvc.SelectProvider(context.Background(), providerName)
		if err != nil {
			return localCommandResultMsg{err: err}
		}
		return localCommandResultMsg{
			notice: fmt.Sprintf("[System] Current provider switched to %s.", selection.ProviderID),
		}
	}
}

func runModelSelection(providerSvc ProviderController, modelID string) tea.Cmd {
	return func() tea.Msg {
		selection, err := providerSvc.SetCurrentModel(context.Background(), modelID)
		if err != nil {
			return localCommandResultMsg{err: err}
		}
		return localCommandResultMsg{
			notice: fmt.Sprintf("[System] Current model switched to %s.", selection.ModelID),
		}
	}
}

func runLocalCommand(configManager *config.Manager, providerSvc ProviderController, raw string) tea.Cmd {
	return func() tea.Msg {
		notice, err := executeLocalCommand(context.Background(), configManager, providerSvc, raw)
		return localCommandResultMsg{notice: notice, err: err}
	}
}

func executeLocalCommand(ctx context.Context, configManager *config.Manager, providerSvc ProviderController, raw string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty command")
	}

	switch strings.ToLower(fields[0]) {
	case slashCommandHelp:
		return slashHelpText(), nil
	case slashCommandStatus:
		return executeStatusCommand(ctx, configManager)
	case slashCommandRun:
		return executeRunCommand(ctx, configManager, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), slashCommandRun)))
	case slashCommandGit:
		return executeGitCommand(ctx, configManager, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), slashCommandGit)))
	case slashCommandFile:
		return executeFileCommand(ctx, configManager, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), slashCommandFile)))
	case slashCommandPlan:
		return buildGenericPlanNotice(), nil
	case slashCommandProvider:
		return executeProviderCommand(ctx, providerSvc, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), slashCommandProvider)))
	case slashCommandSetting:
		return executeSettingCommand(ctx, configManager, providerSvc, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), slashCommandSetting)))
	case slashCommandSet:
		return executeSetCommand(ctx, configManager, providerSvc, fields)
	default:
		return "", fmt.Errorf("unknown command %q", fields[0])
	}
}

func executeSetCommand(ctx context.Context, configManager *config.Manager, providerSvc ProviderController, fields []string) (string, error) {
	if len(fields) < 3 {
		return "", fmt.Errorf("usage: %s | %s | %s", slashUsageSetURL, slashUsageSetKey, slashUsageModel)
	}

	value := strings.TrimSpace(strings.Join(fields[2:], " "))
	if value == "" {
		return "", fmt.Errorf("command value is empty")
	}

	switch strings.ToLower(fields[1]) {
	case "url":
		if _, err := url.ParseRequestURI(value); err != nil {
			return "", fmt.Errorf("invalid url: %w", err)
		}
		if err := configManager.Update(ctx, func(cfg *config.Config) error {
			selectedName := strings.TrimSpace(cfg.SelectedProvider)
			for i := range cfg.Providers {
				if strings.EqualFold(strings.TrimSpace(cfg.Providers[i].Name), selectedName) {
					cfg.Providers[i].BaseURL = value
					return nil
				}
			}
			return fmt.Errorf("selected provider %q not found", cfg.SelectedProvider)
		}); err != nil {
			return "", err
		}
		cfg := configManager.Get()
		return fmt.Sprintf("[System] Base URL updated for %s -> %s", cfg.SelectedProvider, value), nil
	case "key":
		cfg := configManager.Get()
		selected, err := cfg.SelectedProviderConfig()
		if err != nil {
			return "", err
		}
		if err := os.Setenv(selected.APIKeyEnv, value); err != nil {
			return "", fmt.Errorf("set api key env: %w", err)
		}
		return fmt.Sprintf("[System] %s updated for the current process.", selected.APIKeyEnv), nil
	case "model":
		selection, err := providerSvc.SetCurrentModel(ctx, value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[System] Current model switched to %s.", selection.ModelID), nil
	default:
		return "", fmt.Errorf("unsupported /set field %q", fields[1])
	}
}

func executeStatusCommand(ctx context.Context, configManager *config.Manager) (string, error) {
	cfg := configManager.Get()
	lines := []string{
		"Workspace status:",
		"Workdir: " + cfg.Workdir,
		"Provider: " + cfg.SelectedProvider,
		"Model: " + cfg.CurrentModel,
		"Config: " + configManager.ConfigPath(),
	}

	gitStatus, err := shellCommandExecutor(ctx, cfg, "git status --short --branch")
	if err != nil {
		lines = append(lines, "Git: unavailable ("+err.Error()+")")
	} else {
		lines = append(lines, "Git:")
		lines = append(lines, indentBlock(gitStatus, "  "))
	}

	return strings.Join(lines, "\n"), nil
}

func executeRunCommand(ctx context.Context, configManager *config.Manager, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("usage: %s", slashUsageRun)
	}

	cfg := configManager.Get()
	output, err := shellCommandExecutor(ctx, cfg, command)
	if err != nil {
		return "", err
	}
	return "Run output:\n" + indentBlock(output, "  "), nil
}

func executeGitCommand(ctx context.Context, configManager *config.Manager, args string) (string, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return "", fmt.Errorf("usage: %s", slashUsageGit)
	}

	cfg := configManager.Get()
	output, err := shellCommandExecutor(ctx, cfg, "git "+args)
	if err != nil {
		return "", err
	}
	return "Git output:\n" + indentBlock(output, "  "), nil
}

func executeFileCommand(ctx context.Context, configManager *config.Manager, args string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	cfg := configManager.Get()
	subcommand, remainder := splitFirstWord(args)
	switch strings.ToLower(subcommand) {
	case "read":
		path, _ := splitFirstWord(remainder)
		if strings.TrimSpace(path) == "" {
			return "", fmt.Errorf("usage: %s", slashUsageFile)
		}
		target, err := resolveWorkspacePath(cfg.Workdir, path)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("File: %s\n%s", target, string(data)), nil
	case "write":
		path, content := splitFirstWord(remainder)
		if strings.TrimSpace(path) == "" || strings.TrimSpace(content) == "" {
			return "", fmt.Errorf("usage: /file write <path> <content>")
		}
		target, err := resolveWorkspacePath(cfg.Workdir, path)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("Wrote %d bytes to %s", len(content), target), nil
	case "list":
		path := strings.TrimSpace(remainder)
		if path == "" {
			path = "."
		}
		target, err := resolveWorkspacePath(cfg.Workdir, path)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(target)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return fmt.Sprintf("File: %s (%d bytes)", target, info.Size()), nil
		}
		entries, err := os.ReadDir(target)
		if err != nil {
			return "", err
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += string(os.PathSeparator)
			}
			names = append(names, name)
		}
		sort.Strings(names)
		if len(names) == 0 {
			return fmt.Sprintf("Directory %s is empty", target), nil
		}
		return fmt.Sprintf("Files in %s:\n%s", target, indentBlock(strings.Join(names, "\n"), "  ")), nil
	default:
		return "", fmt.Errorf("usage: %s", slashUsageFile)
	}
}

func executeSettingCommand(ctx context.Context, configManager *config.Manager, providerSvc ProviderController, args string) (string, error) {
	cfg := configManager.Get()
	subcommand, remainder := splitFirstWord(args)
	if subcommand == "" {
		return strings.Join([]string{
			"Settings:",
			"Provider: " + cfg.SelectedProvider,
			"Model: " + cfg.CurrentModel,
			"Workdir: " + cfg.Workdir,
			"Shell: " + cfg.Shell,
			"Config: " + configManager.ConfigPath(),
		}, "\n"), nil
	}

	value := strings.TrimSpace(remainder)
	if value == "" {
		return "", fmt.Errorf("usage: %s", slashUsageSetting)
	}

	switch strings.ToLower(subcommand) {
	case "model":
		selection, err := providerSvc.SetCurrentModel(ctx, value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[System] Current model switched to %s.", selection.ModelID), nil
	case "workdir":
		next, err := resolveRequestedWorkdir(cfg.Workdir, value)
		if err != nil {
			return "", err
		}
		if err := configManager.Update(ctx, func(cfg *config.Config) error {
			cfg.Workdir = next
			return nil
		}); err != nil {
			return "", err
		}
		if _, err := configManager.Reload(ctx); err != nil {
			return "", fmt.Errorf("reload config: %w", err)
		}
		return "[System] Workdir updated to " + next, nil
	case "provider":
		return executeProviderCommand(ctx, providerSvc, value)
	default:
		return "", fmt.Errorf("unsupported /setting field %q", subcommand)
	}
}

func executeProviderCommand(ctx context.Context, providerSvc ProviderController, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("usage: %s", slashUsageProvider)
	}
	selection, err := providerSvc.SelectProvider(ctx, value)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[System] Current provider switched to %s.", selection.ProviderID), nil
}

func slashHelpText() string {
	lines := []string{"Available slash commands:"}
	for _, command := range builtinSlashCommands {
		lines = append(lines, fmt.Sprintf("%s - %s", command.Usage, command.Description))
	}
	return strings.Join(lines, "\n")
}

func buildGenericPlanNotice() string {
	return strings.Join([]string{
		"Suggested plan:",
		"1. Inspect the relevant files and current state.",
		"2. Confirm the smallest safe change set.",
		"3. Implement the change.",
		"4. Run verification and review the output.",
	}, "\n")
}

func splitFirstWord(input string) (string, string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ""
	}
	index := strings.IndexAny(input, " \t")
	if index < 0 {
		return input, ""
	}
	return input[:index], strings.TrimSpace(input[index+1:])
}

func indentBlock(text string, prefix string) string {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n")
	if text == "" {
		return prefix + "(no output)"
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func defaultShellCommandExecutor(ctx context.Context, cfg config.Config, command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New("command is empty")
	}

	timeoutSec := cfg.ToolTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = config.DefaultToolTimeoutSec
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	args := shellArgs(cfg.Shell, command)
	cmd := exec.CommandContext(runCtx, args[0], args[1:]...)
	cmd.Dir = cfg.Workdir
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if runCtx.Err() == context.DeadlineExceeded {
		if text == "" {
			return "", fmt.Errorf("command timed out after %ds", timeoutSec)
		}
		return "", fmt.Errorf("command timed out after %ds\n%s", timeoutSec, text)
	}
	if err != nil {
		if text == "" {
			return "", err
		}
		return "", fmt.Errorf("%w\n%s", err, text)
	}
	if text == "" {
		return "(no output)", nil
	}
	return text, nil
}

func shellArgs(shell string, command string) []string {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "powershell", "pwsh":
		return []string{"powershell", "-NoProfile", "-Command", command}
	case "bash":
		return []string{"bash", "-lc", command}
	case "sh":
		return []string{"sh", "-lc", command}
	default:
		return []string{"powershell", "-NoProfile", "-Command", command}
	}
}

func resolveWorkspacePath(root string, requested string) (string, error) {
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	target := strings.TrimSpace(requested)
	if target == "" {
		return "", errors.New("path is empty")
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(base, target)
	}

	target, err = filepath.Abs(target)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("path escapes workspace root")
	}
	return target, nil
}

func resolveRequestedWorkdir(current string, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", errors.New("workdir is empty")
	}
	if filepath.IsAbs(requested) {
		return filepath.Clean(requested), nil
	}
	return filepath.Abs(filepath.Join(current, requested))
}
