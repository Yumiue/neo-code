package decider

import (
	"testing"

	runtimefacts "neo-code/internal/runtime/facts"
)

func TestContinueWithCompletionReasonBranches(t *testing.T) {
	t.Parallel()

	t.Run("pending_todo without open ids requires input", func(t *testing.T) {
		t.Parallel()
		decision := continueWithCompletionReason(DecisionInput{
			CompletionReason: "pending_todo",
			Todos: TodoSnapshot{
				Items: []TodoViewItem{{ID: "x", Required: true, Status: "completed"}},
			},
		})
		if decision.RequiredInput == nil || decision.RequiredInput.Kind != "missing_required_todo_id" {
			t.Fatalf("required input = %+v", decision.RequiredInput)
		}
	})

	t.Run("unverified_write without target requires input", func(t *testing.T) {
		t.Parallel()
		decision := continueWithCompletionReason(DecisionInput{
			CompletionReason: "unverified_write",
		})
		if decision.RequiredInput == nil || decision.RequiredInput.Kind != "missing_file_target_or_content" {
			t.Fatalf("required input = %+v", decision.RequiredInput)
		}
	})

	t.Run("post_execute_closure_required maps to closure missing fact", func(t *testing.T) {
		t.Parallel()
		decision := continueWithCompletionReason(DecisionInput{
			CompletionReason: "post_execute_closure_required",
		})
		if len(decision.MissingFacts) == 0 || decision.MissingFacts[0].Kind != "post_execute_closure" {
			t.Fatalf("missing facts = %+v", decision.MissingFacts)
		}
	})
}

func TestDecideTaskSpecificBranches(t *testing.T) {
	t.Parallel()

	t.Run("todo_state without creation facts requests todo_write", func(t *testing.T) {
		t.Parallel()
		decision := decideTodoState(DecisionInput{})
		if decision.RequiredInput == nil || decision.RequiredInput.Kind != "missing_todo_content" {
			t.Fatalf("required input = %+v", decision.RequiredInput)
		}
	})

	t.Run("mixed accepts when verification passed and required todos closed", func(t *testing.T) {
		t.Parallel()
		decision := decideMixed(DecisionInput{
			Facts: runtimefacts.RuntimeFacts{
				Verification: runtimefacts.VerificationFacts{
					Passed: []runtimefacts.VerificationFact{{Tool: "filesystem_read_file", Scope: "artifact:a.txt", Passed: true}},
				},
			},
			Todos: TodoSnapshot{Summary: TodoSummary{RequiredOpen: 0}},
		})
		if decision.Status != DecisionAccepted {
			t.Fatalf("status = %q, want accepted", decision.Status)
		}
	})

	t.Run("subagent failed without completion returns failed", func(t *testing.T) {
		t.Parallel()
		decision := decideSubAgent(DecisionInput{
			Facts: runtimefacts.RuntimeFacts{
				SubAgents: runtimefacts.SubAgentFacts{
					Started: []runtimefacts.SubAgentFact{{TaskID: "sa-1"}},
					Failed:  []runtimefacts.SubAgentFact{{TaskID: "sa-1"}},
				},
			},
		})
		if decision.Status != DecisionFailed {
			t.Fatalf("status = %q, want failed", decision.Status)
		}
	})
}

func TestHelperFunctionsBranches(t *testing.T) {
	t.Parallel()

	if got := latestToolErrorDetail([]runtimefacts.ToolErrorFact{{Tool: "filesystem_write_file", ErrorClass: "timeout"}}, "filesystem_write_file"); got != "timeout" {
		t.Fatalf("latestToolErrorDetail = %q, want timeout", got)
	}

	if !hasVerificationForTarget(runtimefacts.RuntimeFacts{
		Verification: runtimefacts.VerificationFacts{
			Passed: []runtimefacts.VerificationFact{{Scope: "artifact:./docs/../docs/readme.md", Passed: true}},
		},
	}, "docs/readme.md") {
		t.Fatal("expected normalized artifact scope match")
	}

	if got := extractGoalPaths(`edit "A.TXT", then update a.txt and b.md`); len(got) != 2 {
		t.Fatalf("extractGoalPaths len = %d, want 2", len(got))
	}
}

