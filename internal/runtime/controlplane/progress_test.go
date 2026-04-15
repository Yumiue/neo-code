package controlplane

import "testing"

func TestApplyProgressEvidenceNoEvidenceIncrementsNoProgress(t *testing.T) {
	t.Parallel()
	state := ProgressState{}
	next := ApplyProgressEvidence(state, nil, "")
	if next.LastScore.NoProgressStreak != 1 {
		t.Fatalf("expected no_progress_streak 1, got %d", next.LastScore.NoProgressStreak)
	}
}

func TestApplyProgressEvidenceOnlyNonDupResetsNoProgressStreak(t *testing.T) {
	t.Parallel()
	state := ProgressState{
		LastScore: ProgressScore{NoProgressStreak: 3},
	}
	next := ApplyProgressEvidence(state, []ProgressEvidenceRecord{
		{Kind: EvidenceNewInfoNonDup},
	}, "sig1")
	if next.LastScore.NoProgressStreak != 0 {
		t.Fatalf("expected streak reset to 0, got %d", next.LastScore.NoProgressStreak)
	}
	if next.LastScore.ScoreDelta != 1 {
		t.Fatalf("expected score_delta 1, got %d", next.LastScore.ScoreDelta)
	}
}

func TestApplyProgressEvidenceMixedResetsNoProgress(t *testing.T) {
	t.Parallel()
	state := ProgressState{
		LastScore: ProgressScore{NoProgressStreak: 2},
	}
	next := ApplyProgressEvidence(state, []ProgressEvidenceRecord{
		{Kind: EvidenceNewInfoNonDup},
		{Kind: ProgressEvidenceKind("other_evidence")},
	}, "sig1")
	if next.LastScore.NoProgressStreak != 0 {
		t.Fatalf("expected streak reset, got %d", next.LastScore.NoProgressStreak)
	}
}

func TestApplyProgressEvidenceRepeatCycle(t *testing.T) {
	t.Parallel()
	state := ProgressState{
		LastScore:     ProgressScore{NoProgressStreak: 1, RepeatCycleStreak: 1},
		LastSignature: "sig1",
	}
	next := ApplyProgressEvidence(state, []ProgressEvidenceRecord{
		{Kind: EvidenceNewInfoNonDup},
	}, "sig1")
	if next.LastScore.NoProgressStreak != 2 {
		t.Fatalf("expected no_progress_streak 2, got %d", next.LastScore.NoProgressStreak)
	}
	if next.LastScore.RepeatCycleStreak != 2 {
		t.Fatalf("expected repeat_cycle_streak 2, got %d", next.LastScore.RepeatCycleStreak)
	}
	if next.LastSignature != "sig1" {
		t.Fatalf("expected signature sig1, got %s", next.LastSignature)
	}
}
