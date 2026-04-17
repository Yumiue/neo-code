package security

import (
	"strings"
	"time"
)

// EvaluateCapabilityForEngine 在权限引擎入口执行 capability 判定。
// 当 action 未携带 capability token 时，第二个返回值为 false。
func EvaluateCapabilityForEngine(action Action, now time.Time) (CheckResult, bool) {
	token := action.Payload.CapabilityToken
	if token == nil {
		return CheckResult{}, false
	}

	allowed, reason := EvaluateCapabilityAction(*token, action, now)
	if allowed {
		return CheckResult{}, false
	}
	return capabilityDeniedResult(action, reason), true
}

// IsCapabilityDeniedResult 判断权限结果是否由 capability 拒绝产生。
func IsCapabilityDeniedResult(result CheckResult) bool {
	if result.Rule == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(result.Rule.ID), CapabilityRuleID)
}

func capabilityDeniedResult(action Action, reason string) CheckResult {
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = "capability token denied"
	}
	return CheckResult{
		Decision: DecisionDeny,
		Action:   action,
		Rule: &Rule{
			ID:       CapabilityRuleID,
			Type:     action.Type,
			Resource: action.Payload.Resource,
			Decision: DecisionDeny,
			Reason:   trimmedReason,
		},
		Reason: trimmedReason,
	}
}
