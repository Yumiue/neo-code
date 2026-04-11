package context

import "strings"

type promptSection struct {
	Title   string
	Content string
}

// PromptSection 是 promptSection 的导出版本，允许外部包构造 prompt section。
type PromptSection = promptSection

// NewPromptSection 创建一个 promptSection 实例。
func NewPromptSection(title, content string) promptSection {
	return promptSection{Title: title, Content: content}
}

var defaultPromptSections = []promptSection{
	{
		Title: "Agent Identity",
		Content: "You are NeoCode, a local coding agent focused on completing the current task end-to-end.\n" +
			"Preserve the main loop of user input, agent reasoning, tool execution, result observation, and UI feedback.",
	},
	{
		Title: "Tool Usage",
		Content: "- Use tools when they reduce uncertainty or are required to complete the task safely.\n" +
			"- For risky operations, call the relevant tool first and let the runtime permission layer decide ask/allow/deny.\n" +
			"- Do not self-reject a user-requested operation before attempting the proper tool call and permission flow.\n" +
			"- Stay within the current workspace unless the user clearly asks for something else.\n" +
			"- Do not claim work is done unless the needed files, commands, or verification actually succeeded.",
	},
	{
		Title: "Failure Recovery",
		Content: "- If blocked, identify the concrete blocker and try the next reasonable path before giving up.\n" +
			"- Surface risky assumptions, partial progress, or missing verification instead of hiding them.\n" +
			"- When constraints prevent completion, return the best safe result and explain what remains.",
	},
	{
		Title: "Response Style",
		Content: "- Be concise, accurate, and collaborative.\n" +
			"- Keep updates focused on useful progress, decisions, and verification.\n" +
			"- Base claims on the current workspace state instead of generic advice.",
	},
}

func defaultSystemPromptSections() []promptSection {
	return defaultPromptSections
}

func composeSystemPrompt(sections ...promptSection) string {
	rendered := make([]string, 0, len(sections))
	for _, section := range sections {
		part := renderPromptSection(section)
		if part == "" {
			continue
		}
		rendered = append(rendered, part)
	}
	return strings.Join(rendered, "\n\n")
}

func renderPromptSection(section promptSection) string {
	title := strings.TrimSpace(section.Title)
	content := strings.TrimSpace(section.Content)

	switch {
	case title == "" && content == "":
		return ""
	case title == "":
		return content
	case content == "":
		return ""
	default:
		var builder strings.Builder
		builder.Grow(len(title) + len(content) + len("## \n\n"))
		builder.WriteString("## ")
		builder.WriteString(title)
		builder.WriteString("\n\n")
		builder.WriteString(content)
		return builder.String()
	}
}
