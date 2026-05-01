package ptyproxy

import (
	"strings"
	"testing"
)

func TestMergeEnvVarOverridesExistingValue(t *testing.T) {
	merged := MergeEnvVar([]string{
		"PATH=/bin",
		"NEOCODE_DIAG_SOCKET=/tmp/old.sock",
		"HOME=/home/tester",
	}, DiagSocketEnv, "/tmp/new.sock")

	var socketEntries []string
	for _, item := range merged {
		if strings.HasPrefix(item, DiagSocketEnv+"=") {
			socketEntries = append(socketEntries, item)
		}
	}
	if len(socketEntries) != 1 {
		t.Fatalf("socket entries len = %d, want 1", len(socketEntries))
	}
	if socketEntries[0] != DiagSocketEnv+"=/tmp/new.sock" {
		t.Fatalf("socket entry = %q", socketEntries[0])
	}
}
