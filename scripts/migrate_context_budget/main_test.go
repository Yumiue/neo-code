package main

import (
	"testing"
)

func TestDefaultBaseDirReturnsPath(t *testing.T) {
	t.Parallel()

	if got := defaultBaseDir(); got == "" {
		t.Fatal("expected non-empty default base dir")
	}
}
