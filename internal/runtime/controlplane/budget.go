package controlplane

// TurnBudgetAction 表示预算控制面对单次发送尝试做出的唯一动作。
type TurnBudgetAction string

const (
	TurnBudgetActionAllow   TurnBudgetAction = "allow"
	TurnBudgetActionCompact TurnBudgetAction = "compact"
	TurnBudgetActionStop    TurnBudgetAction = "stop"
)

// TurnBudgetID 标识一次冻结预算尝试，避免 estimate、decision 与 usage observation 串用。
type TurnBudgetID struct {
	AttemptSeq  int    `json:"attempt_seq"`
	RequestHash string `json:"request_hash"`
}

// TurnBudgetEstimate 描述 runtime 对冻结请求输入 token 的主干估算事实。
type TurnBudgetEstimate struct {
	ID                   TurnBudgetID `json:"id"`
	EstimatedInputTokens int          `json:"estimated_input_tokens"`
	EstimateSource       string       `json:"estimate_source,omitempty"`
	Accurate             bool         `json:"accurate"`
}

// TurnBudgetDecision 描述冻结请求在当前预算事实下的决策结果。
type TurnBudgetDecision struct {
	ID                   TurnBudgetID     `json:"id"`
	Action               TurnBudgetAction `json:"action"`
	Reason               string           `json:"reason,omitempty"`
	EstimatedInputTokens int              `json:"estimated_input_tokens"`
	PromptBudget         int              `json:"prompt_budget"`
	EstimateSource       string           `json:"estimate_source,omitempty"`
}

// DecideTurnBudget 根据输入预算事实输出 allow、compact 或 stop 三种动作。
func DecideTurnBudget(
	estimate TurnBudgetEstimate,
	promptBudget int,
	compactCount int,
) TurnBudgetDecision {
	decision := TurnBudgetDecision{
		ID:                   estimate.ID,
		EstimatedInputTokens: estimate.EstimatedInputTokens,
		PromptBudget:         promptBudget,
		EstimateSource:       estimate.EstimateSource,
	}
	if estimate.EstimatedInputTokens <= promptBudget {
		decision.Action = TurnBudgetActionAllow
		decision.Reason = "within_budget"
		return decision
	}
	if compactCount == 0 {
		decision.Action = TurnBudgetActionCompact
		decision.Reason = "exceeds_budget_first_time"
		return decision
	}
	decision.Action = TurnBudgetActionStop
	decision.Reason = "exceeds_budget_after_compact"
	return decision
}
