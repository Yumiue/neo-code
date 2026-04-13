package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"neo-code/internal/config"
	configstate "neo-code/internal/config/state"
	agentcontext "neo-code/internal/context"
	"neo-code/internal/memo"
	"neo-code/internal/provider"
	"neo-code/internal/provider/builtin"
	providercatalog "neo-code/internal/provider/catalog"
	providertypes "neo-code/internal/provider/types"
	agentruntime "neo-code/internal/runtime"
	"neo-code/internal/security"
	agentsession "neo-code/internal/session"
	"neo-code/internal/tools"
	"neo-code/internal/tools/bash"
	"neo-code/internal/tools/filesystem"
	"neo-code/internal/tools/mcp"
	memotool "neo-code/internal/tools/memo"
	"neo-code/internal/tools/webfetch"
	"neo-code/internal/tui"
)

const utf8CodePage = 65001

var (
	setConsoleOutputCodePage = platformSetConsoleOutputCodePage
	setConsoleInputCodePage  = platformSetConsoleInputCodePage
)

// BootstrapOptions 描述应用启动时可注入的运行时选项。
type BootstrapOptions struct {
	Workdir string
}

// RuntimeBundle 聚合 CLI 与 TUI 共享的运行时依赖。
type RuntimeBundle struct {
	Config            config.Config
	ConfigManager     *config.Manager
	Runtime           agentruntime.Runtime
	ProviderSelection *configstate.Service
	MemoService       *memo.Service
}

// EnsureConsoleUTF8 负责在 Windows 控制台中尽量启用 UTF-8 编码。
func EnsureConsoleUTF8() {
	if err := setConsoleOutputCodePage(utf8CodePage); err != nil {
		return
	}
	_ = setConsoleInputCodePage(utf8CodePage)
}

// BuildRuntime 构建 CLI 与 TUI 共用的运行时依赖。
func BuildRuntime(ctx context.Context, opts BootstrapOptions) (RuntimeBundle, error) {
	defaultCfg, err := bootstrapDefaultConfig(opts)
	if err != nil {
		return RuntimeBundle{}, err
	}

	loader := config.NewLoader("", defaultCfg)
	manager := config.NewManager(loader)
	if _, err := manager.Load(ctx); err != nil {
		return RuntimeBundle{}, err
	}

	providerRegistry, err := builtin.NewRegistry()
	if err != nil {
		return RuntimeBundle{}, err
	}
	modelCatalogs := providercatalog.NewService(manager.BaseDir(), providerRegistry, nil)
	providerSelection := configstate.NewService(manager, providerRegistry, modelCatalogs)
	if _, err := providerSelection.EnsureSelection(ctx); err != nil {
		return RuntimeBundle{}, err
	}

	cfg := manager.Get()

	toolRegistry, err := buildToolRegistry(cfg)
	if err != nil {
		return RuntimeBundle{}, err
	}
	toolManager, err := buildToolManager(toolRegistry)
	if err != nil {
		return RuntimeBundle{}, err
	}

	// Session Store 绑定到启动时的 workdir 哈希分桶，整个应用生命周期内不可变。
	// 这意味着所有会话都归属到启动时指定的项目目录下，运行时不会因配置变更而迁移存储位置。
	sessionStore := agentsession.NewStore(loader.BaseDir(), cfg.Workdir)

	var contextBuilder agentcontext.Builder = agentcontext.NewBuilderWithToolPolicies(toolRegistry)
	var memoSvc *memo.Service
	if cfg.Memo.Enabled {
		memoStore := memo.NewFileStore(loader.BaseDir(), cfg.Workdir)
		memoSource := memo.NewContextSource(memoStore)
		var sourceInvl func()
		if invalidator, ok := memoSource.(interface{ InvalidateCache() }); ok {
			sourceInvl = invalidator.InvalidateCache
		}
		contextBuilder = agentcontext.NewBuilderWithMemo(toolRegistry, memoSource)
		memoSvc = memo.NewService(memoStore, nil, cfg.Memo, sourceInvl)
		toolRegistry.Register(memotool.NewRememberTool(memoSvc))
		toolRegistry.Register(memotool.NewRecallTool(memoSvc))
	}

	runtimeSvc := agentruntime.NewWithFactory(
		manager,
		toolManager,
		sessionStore,
		providerRegistry,
		contextBuilder,
	)

	// 注入记忆提取钩子：当 AutoExtract 启用且 memoSvc 可用时，ReAct 循环完成后异步提取记忆。
	if memoSvc != nil && cfg.Memo.AutoExtract {
		runtimeSvc.SetMemoExtractor(newMemoExtractorAdapter(
			providerRegistry,
			manager,
			memo.NewAutoExtractor(nil, memoSvc),
		))
	}

	return RuntimeBundle{
		Config:            cfg,
		ConfigManager:     manager,
		Runtime:           runtimeSvc,
		ProviderSelection: providerSelection,
		MemoService:       memoSvc,
	}, nil
}

// NewProgram 基于共享运行时依赖构建并返回 TUI 程序。
func NewProgram(ctx context.Context, opts BootstrapOptions) (*tea.Program, error) {
	bundle, err := BuildRuntime(ctx, opts)
	if err != nil {
		return nil, err
	}

	tuiApp, err := tui.NewWithMemo(&bundle.Config, bundle.ConfigManager, bundle.Runtime, bundle.ProviderSelection, bundle.MemoService)
	if err != nil {
		return nil, err
	}
	return tea.NewProgram(
		tuiApp,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	), nil
}

// bootstrapDefaultConfig 负责计算本次启动应使用的默认配置快照。
func bootstrapDefaultConfig(opts BootstrapOptions) (*config.Config, error) {
	defaultCfg := config.StaticDefaults()
	workdir := strings.TrimSpace(opts.Workdir)
	if workdir == "" {
		return defaultCfg, nil
	}

	resolved, err := resolveBootstrapWorkdir(workdir)
	if err != nil {
		return nil, err
	}
	defaultCfg.Workdir = resolved
	return defaultCfg, nil
}

// resolveBootstrapWorkdir 将 CLI 传入的工作区解析为存在的绝对目录。
func resolveBootstrapWorkdir(workdir string) (string, error) {
	return agentsession.ResolveExistingDir(workdir)
}

func buildToolRegistry(cfg config.Config) (*tools.Registry, error) {
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(filesystem.New(cfg.Workdir))
	toolRegistry.Register(filesystem.NewWrite(cfg.Workdir))
	toolRegistry.Register(filesystem.NewGrep(cfg.Workdir))
	toolRegistry.Register(filesystem.NewGlob(cfg.Workdir))
	toolRegistry.Register(filesystem.NewEdit(cfg.Workdir))
	toolRegistry.Register(bash.New(cfg.Workdir, cfg.Shell, time.Duration(cfg.ToolTimeoutSec)*time.Second))
	toolRegistry.Register(webfetch.New(webfetch.Config{
		Timeout:               time.Duration(cfg.ToolTimeoutSec) * time.Second,
		MaxResponseBytes:      cfg.Tools.WebFetch.MaxResponseBytes,
		SupportedContentTypes: cfg.Tools.WebFetch.SupportedContentTypes,
	}))
	mcpRegistry, err := buildMCPRegistry(cfg)
	if err != nil {
		return nil, err
	}
	if mcpRegistry != nil {
		toolRegistry.SetMCPRegistry(mcpRegistry)
		toolRegistry.SetMCPExposureFilter(mcp.NewExposureFilter(mcp.ExposureFilterConfig{
			Allowlist: cfg.Tools.MCP.Exposure.Allowlist,
			Denylist:  cfg.Tools.MCP.Exposure.Denylist,
			Agents:    buildMCPAgentExposureRules(cfg.Tools.MCP.Exposure.Agents),
		}))
	}
	return toolRegistry, nil
}

// buildMCPAgentExposureRules 将配置层的 agent 过滤规则转换为 tools/mcp 层输入。
func buildMCPAgentExposureRules(configs []config.MCPAgentExposureConfig) []mcp.AgentExposureRule {
	if len(configs) == 0 {
		return nil
	}
	rules := make([]mcp.AgentExposureRule, 0, len(configs))
	for _, item := range configs {
		rules = append(rules, mcp.AgentExposureRule{
			Agent:     item.Agent,
			Allowlist: append([]string(nil), item.Allowlist...),
		})
	}
	return rules
}

func buildToolManager(registry *tools.Registry) (tools.Manager, error) {
	engine, err := security.NewRecommendedPolicyEngine()
	if err != nil {
		return nil, err
	}
	return tools.NewManager(registry, engine, security.NewWorkspaceSandbox())
}

type memoExtractorScheduler interface {
	ScheduleWithExtractor(sessionID string, messages []providertypes.Message, extractor memo.Extractor)
}

// memoExtractorAdapter 在调度时绑定 provider 配置快照，避免后台任务读取全局可变配置。
type memoExtractorAdapter struct {
	factory       agentruntime.ProviderFactory
	configManager *config.Manager
	scheduler     memoExtractorScheduler
}

// newMemoExtractorAdapter 创建绑定当前 provider 选择的记忆提取调度适配器。
func newMemoExtractorAdapter(
	factory agentruntime.ProviderFactory,
	cm *config.Manager,
	scheduler memoExtractorScheduler,
) *memoExtractorAdapter {
	return &memoExtractorAdapter{
		factory:       factory,
		configManager: cm,
		scheduler:     scheduler,
	}
}

// Schedule 在当前运行结束时绑定 provider/model 快照，再交给后台调度器延后执行。
func (a *memoExtractorAdapter) Schedule(sessionID string, messages []providertypes.Message) {
	if a == nil || a.scheduler == nil {
		return
	}

	cfg := a.configManager.Get()
	resolved, err := config.ResolveSelectedProvider(cfg)
	if err != nil {
		log.Printf("memo: resolve selected provider failed: %v", err)
		return
	}

	extractor := memo.NewLLMExtractor(newProviderTextGenerator(
		a.factory,
		resolved.ToRuntimeConfig(),
		cfg.CurrentModel,
	))
	a.scheduler.ScheduleWithExtractor(sessionID, messages, extractor)
}

// providerTextGenerator 复用当前 provider 配置快照，向 memo 提供纯文本生成能力。
type providerTextGenerator struct {
	factory    agentruntime.ProviderFactory
	runtimeCfg provider.RuntimeConfig
	model      string
}

// newProviderTextGenerator 创建绑定固定 provider/model 快照的文本生成适配器。
func newProviderTextGenerator(
	factory agentruntime.ProviderFactory,
	runtimeCfg provider.RuntimeConfig,
	model string,
) *providerTextGenerator {
	return &providerTextGenerator{
		factory:    factory,
		runtimeCfg: runtimeCfg,
		model:      model,
	}
}

// Generate 使用预先绑定的 provider/model 快照发起无工具的独立生成请求。
func (g *providerTextGenerator) Generate(
	ctx context.Context,
	prompt string,
	messages []providertypes.Message,
) (string, error) {
	modelProvider, err := g.factory.Build(ctx, g.runtimeCfg)
	if err != nil {
		return "", err
	}

	events := make(chan providertypes.StreamEvent, 32)
	done := make(chan error, 1)
	var builder strings.Builder

	go func() {
		messageDone := false
		var streamErr error
		for event := range events {
			switch event.Type {
			case providertypes.StreamEventTextDelta:
				payload, err := event.TextDeltaValue()
				if err != nil {
					if streamErr == nil {
						streamErr = err
					}
					continue
				}
				builder.WriteString(payload.Text)
			case providertypes.StreamEventMessageDone:
				if _, err := event.MessageDoneValue(); err != nil {
					if streamErr == nil {
						streamErr = err
					}
					continue
				}
				messageDone = true
			default:
				if streamErr == nil {
					streamErr = fmt.Errorf("memo: unexpected provider stream event %q", event.Type)
				}
			}
		}
		if streamErr == nil && !messageDone {
			streamErr = fmt.Errorf("memo: provider stream ended without message_done event")
		}
		done <- streamErr
	}()

	err = modelProvider.Generate(ctx, providertypes.GenerateRequest{
		Model:        g.model,
		SystemPrompt: prompt,
		Messages:     append([]providertypes.Message(nil), messages...),
	}, events)
	close(events)

	streamErr := <-done
	if streamErr != nil {
		if err != nil {
			return "", fmt.Errorf("memo: provider generate failed: %v: %w", streamErr, err)
		}
		return "", streamErr
	}
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}
