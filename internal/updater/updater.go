package updater

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"

	selfupdate "github.com/creativeprojects/go-selfupdate"

	"neo-code/internal/version"
)

const (
	repositoryOwner  = "1024XEngineer"
	repositoryName   = "neo-code"
	checksumFilename = "checksums.txt"
)

var (
	runtimeGOOS   = runtime.GOOS
	runtimeGOARCH = runtime.GOARCH
)

var (
	newClient = func(config selfupdate.Config) (updateClient, error) {
		updater, err := selfupdate.NewUpdater(config)
		if err != nil {
			return nil, err
		}
		return selfupdateClient{updater: updater}, nil
	}
	resolveExecutablePath = selfupdate.ExecutablePath
)

type assetTarget struct {
	OSToken   string
	ArchToken string
	Ext       string
	AssetName string
}

type releaseView interface {
	Version() string
	GreaterThan(other string) bool
}

type updateClient interface {
	DetectLatest(ctx context.Context, repository selfupdate.Repository) (releaseView, bool, error)
	UpdateTo(ctx context.Context, rel releaseView, cmdPath string) error
}

type selfupdateClient struct {
	updater *selfupdate.Updater
}

type selfupdateRelease struct {
	release *selfupdate.Release
}

// CheckOptions 描述静默检测新版本时的输入参数。
type CheckOptions struct {
	CurrentVersion    string
	IncludePrerelease bool
}

// CheckResult 表示静默检测流程返回的版本信息。
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	HasUpdate      bool
}

// UpdateOptions 描述手动更新命令的输入参数。
type UpdateOptions struct {
	CurrentVersion    string
	IncludePrerelease bool
}

// UpdateResult 表示手动更新流程的最终结果。
type UpdateResult struct {
	CurrentVersion string
	LatestVersion  string
	Updated        bool
}

// CheckLatest 按当前平台资产规则检测最新版本，不做本地文件替换。
func CheckLatest(ctx context.Context, opts CheckOptions) (CheckResult, error) {
	currentVersion := normalizeCurrentVersion(opts.CurrentVersion)
	target, err := resolveAssetTarget(runtimeGOOS, runtimeGOARCH)
	if err != nil {
		return CheckResult{CurrentVersion: currentVersion}, err
	}

	client, err := newClient(buildSelfupdateConfig(target, opts.IncludePrerelease))
	if err != nil {
		return CheckResult{CurrentVersion: currentVersion}, err
	}

	repository := selfupdate.NewRepositorySlug(repositoryOwner, repositoryName)
	release, found, err := client.DetectLatest(ctx, repository)
	if err != nil {
		return CheckResult{CurrentVersion: currentVersion}, err
	}

	result := CheckResult{CurrentVersion: currentVersion}
	if !found || release == nil {
		return result, nil
	}

	result.LatestVersion = strings.TrimSpace(release.Version())
	if result.LatestVersion == "" {
		return result, nil
	}

	if version.IsSemverRelease(currentVersion) {
		result.HasUpdate = release.GreaterThan(currentVersion)
	}
	return result, nil
}

// DoUpdate 下载并校验最新版本后原地替换当前可执行文件。
func DoUpdate(ctx context.Context, opts UpdateOptions) (UpdateResult, error) {
	currentVersion := normalizeCurrentVersion(opts.CurrentVersion)
	target, err := resolveAssetTarget(runtimeGOOS, runtimeGOARCH)
	if err != nil {
		return UpdateResult{CurrentVersion: currentVersion}, err
	}

	client, err := newClient(buildSelfupdateConfig(target, opts.IncludePrerelease))
	if err != nil {
		return UpdateResult{CurrentVersion: currentVersion}, err
	}

	repository := selfupdate.NewRepositorySlug(repositoryOwner, repositoryName)
	release, found, err := client.DetectLatest(ctx, repository)
	if err != nil {
		return UpdateResult{CurrentVersion: currentVersion}, err
	}
	if !found || release == nil {
		return UpdateResult{CurrentVersion: currentVersion}, errors.New("updater: no release asset found for current platform")
	}

	latestVersion := strings.TrimSpace(release.Version())
	result := UpdateResult{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
	}

	if version.IsSemverRelease(currentVersion) && !release.GreaterThan(currentVersion) {
		return result, nil
	}

	executablePath, err := resolveExecutablePath()
	if err != nil {
		return result, err
	}

	if err := client.UpdateTo(ctx, release, executablePath); err != nil {
		return result, err
	}

	result.Updated = true
	return result, nil
}

// DetectLatest 调用底层 go-selfupdate 客户端获取最新版本信息。
func (c selfupdateClient) DetectLatest(ctx context.Context, repository selfupdate.Repository) (releaseView, bool, error) {
	release, found, err := c.updater.DetectLatest(ctx, repository)
	if err != nil || !found || release == nil {
		return nil, found, err
	}
	return selfupdateRelease{release: release}, true, nil
}

// UpdateTo 委托 go-selfupdate 完成原地替换流程，不追加平台分支逻辑。
func (c selfupdateClient) UpdateTo(ctx context.Context, rel releaseView, cmdPath string) error {
	typed, ok := rel.(selfupdateRelease)
	if !ok || typed.release == nil {
		return errors.New("updater: unsupported release type")
	}
	return c.updater.UpdateTo(ctx, typed.release, cmdPath)
}

// Version 返回底层 release 的语义化版本字符串。
func (r selfupdateRelease) Version() string {
	return strings.TrimSpace(r.release.Version())
}

// GreaterThan 判断底层 release 是否高于指定版本。
func (r selfupdateRelease) GreaterThan(other string) bool {
	return r.release.GreaterThan(other)
}

// normalizeCurrentVersion 归一化当前版本输入并处理空值回退。
func normalizeCurrentVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "dev"
	}
	return trimmed
}

// buildSelfupdateConfig 构建严格资产匹配与 checksum 校验配置。
func buildSelfupdateConfig(target assetTarget, includePrerelease bool) selfupdate.Config {
	return selfupdate.Config{
		OS:         target.OSToken,
		Arch:       target.ArchToken,
		Filters:    []string{"^" + regexp.QuoteMeta(target.AssetName) + "$"},
		Validator:  &selfupdate.ChecksumValidator{UniqueFilename: checksumFilename},
		Prerelease: includePrerelease,
	}
}

// resolveAssetTarget 按 GoReleaser 产物命名约束生成当前平台目标资产名。
func resolveAssetTarget(goos string, goarch string) (assetTarget, error) {
	var osToken string
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "linux":
		osToken = "Linux"
	case "darwin":
		osToken = "Darwin"
	case "windows":
		osToken = "Windows"
	default:
		return assetTarget{}, fmt.Errorf("updater: unsupported os %q", goos)
	}

	var archToken string
	switch strings.ToLower(strings.TrimSpace(goarch)) {
	case "amd64":
		archToken = "x86_64"
	case "arm64":
		archToken = "arm64"
	default:
		return assetTarget{}, fmt.Errorf("updater: unsupported arch %q", goarch)
	}

	ext := "tar.gz"
	if osToken == "Windows" {
		ext = "zip"
	}

	return assetTarget{
		OSToken:   osToken,
		ArchToken: archToken,
		Ext:       ext,
		AssetName: fmt.Sprintf("neocode_%s_%s.%s", osToken, archToken, ext),
	}, nil
}
