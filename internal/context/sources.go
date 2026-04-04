package context

import "context"

// promptSectionSource 约束单个 prompt section 来源的最小能力，避免 Builder 持有具体细节。
type promptSectionSource interface {
	Sections(ctx context.Context, input BuildInput) ([]promptSection, error)
}

// corePromptSource 只负责提供固定核心 system prompt sections。
type corePromptSource struct{}

// Sections 返回当前轮次都需要注入的固定核心提示。
func (corePromptSource) Sections(ctx context.Context, input BuildInput) ([]promptSection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]promptSection(nil), defaultSystemPromptSections()...), nil
}

// projectRulesSource 只负责发现并渲染项目规则文件。
type projectRulesSource struct{}

// Sections 按当前工作目录向上发现 AGENTS.md，并渲染为统一 section。
func (projectRulesSource) Sections(ctx context.Context, input BuildInput) ([]promptSection, error) {
	rules, err := loadProjectRules(ctx, input.Metadata.Workdir)
	if err != nil {
		return nil, err
	}

	section := renderProjectRulesSection(rules)
	if renderPromptSection(section) == "" {
		return nil, nil
	}
	return []promptSection{section}, nil
}

// systemStateSource 只负责收集并渲染运行时系统摘要。
type systemStateSource struct {
	gitRunner gitCommandRunner
}

// Sections 汇总 workdir、shell、provider、model 与 git 摘要信息。
func (s systemStateSource) Sections(ctx context.Context, input BuildInput) ([]promptSection, error) {
	systemState, err := collectSystemState(ctx, input.Metadata, s.gitRunner)
	if err != nil {
		return nil, err
	}
	return []promptSection{renderSystemStateSection(systemState)}, nil
}
