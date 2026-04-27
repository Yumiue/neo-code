package cli

import (
	"context"
	"time"

	"neo-code/internal/updater"
)

// defaultReleaseProbe 统一封装版本探测的超时控制与参数透传。
func defaultReleaseProbe(
	ctx context.Context,
	currentVersion string,
	includePrerelease bool,
	timeout time.Duration,
) (updater.CheckResult, error) {
	if timeout <= 0 {
		return checkLatestRelease(ctx, updater.CheckOptions{
			CurrentVersion:    currentVersion,
			IncludePrerelease: includePrerelease,
		})
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return checkLatestRelease(checkCtx, updater.CheckOptions{
		CurrentVersion:    currentVersion,
		IncludePrerelease: includePrerelease,
	})
}
