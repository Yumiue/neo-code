package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"neo-code/internal/updater"
)

func TestDefaultReleaseProbePassesOptions(t *testing.T) {
	originalCheckLatest := checkLatestRelease
	t.Cleanup(func() { checkLatestRelease = originalCheckLatest })

	var capturedCurrent string
	var capturedIncludePrerelease bool
	checkLatestRelease = func(ctx context.Context, options updater.CheckOptions) (updater.CheckResult, error) {
		capturedCurrent = options.CurrentVersion
		capturedIncludePrerelease = options.IncludePrerelease
		return updater.CheckResult{
			CurrentVersion: options.CurrentVersion,
			LatestVersion:  "v1.2.0",
			HasUpdate:      true,
		}, nil
	}

	result, err := defaultReleaseProbe(context.Background(), "v1.0.0", true, time.Second)
	if err != nil {
		t.Fatalf("defaultReleaseProbe() error = %v", err)
	}
	if capturedCurrent != "v1.0.0" {
		t.Fatalf("captured current version = %q, want %q", capturedCurrent, "v1.0.0")
	}
	if !capturedIncludePrerelease {
		t.Fatal("expected include prerelease flag to be forwarded")
	}
	if !result.HasUpdate || result.LatestVersion != "v1.2.0" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestDefaultReleaseProbeReturnsContextTimeoutError(t *testing.T) {
	originalCheckLatest := checkLatestRelease
	t.Cleanup(func() { checkLatestRelease = originalCheckLatest })

	checkLatestRelease = func(ctx context.Context, options updater.CheckOptions) (updater.CheckResult, error) {
		<-ctx.Done()
		return updater.CheckResult{}, ctx.Err()
	}

	_, err := defaultReleaseProbe(context.Background(), "v1.0.0", false, 10*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("defaultReleaseProbe() error = %v, want context deadline exceeded", err)
	}
}

func TestDefaultReleaseProbeWithoutTimeoutUsesOriginalContext(t *testing.T) {
	originalCheckLatest := checkLatestRelease
	t.Cleanup(func() { checkLatestRelease = originalCheckLatest })

	var hasDeadline bool
	checkLatestRelease = func(ctx context.Context, options updater.CheckOptions) (updater.CheckResult, error) {
		_, hasDeadline = ctx.Deadline()
		return updater.CheckResult{CurrentVersion: options.CurrentVersion}, nil
	}

	_, err := defaultReleaseProbe(context.Background(), "v1.0.0", false, 0)
	if err != nil {
		t.Fatalf("defaultReleaseProbe() error = %v", err)
	}
	if hasDeadline {
		t.Fatal("expected original context without deadline when timeout <= 0")
	}
}
