package updater

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

type fakeRelease struct {
	version   string
	greaterFn func(string) bool
}

func (r fakeRelease) Version() string {
	return r.version
}

func (r fakeRelease) GreaterThan(other string) bool {
	if r.greaterFn != nil {
		return r.greaterFn(other)
	}
	return false
}

type fakeClient struct {
	release                 releaseView
	found                   bool
	detectErr               error
	updateErr               error
	updateCalls             int
	lastUpdatePath          string
	probeStatus             probeStatus
	probeLatestVersion      string
	probeInstallableVersion string
	probeLatestInstallable  bool
	probeExpectedPattern    string
	probeAvailableAssetSize int
	probeCandidates         []string
}

func (c *fakeClient) ProbeLatest(_ context.Context, _ selfupdate.Repository, target assetTarget) (probeResult, error) {
	if c.detectErr != nil {
		return probeResult{}, c.detectErr
	}

	expectedPattern := c.probeExpectedPattern
	if strings.TrimSpace(expectedPattern) == "" {
		expectedPattern = buildExpectedPattern(target)
	}
	latest := strings.TrimSpace(c.probeLatestVersion)
	if latest == "" && c.release != nil {
		latest = strings.TrimSpace(c.release.Version())
	}
	installable := strings.TrimSpace(c.probeInstallableVersion)
	if installable == "" && c.release != nil {
		installable = strings.TrimSpace(c.release.Version())
	}

	if c.probeStatus != 0 {
		return probeResult{
			Status:               c.probeStatus,
			Release:              c.release,
			LatestVersion:        latest,
			InstallableVersion:   installable,
			LatestInstallable:    c.probeLatestInstallable,
			ExpectedPattern:      expectedPattern,
			AvailableAssetsCount: c.probeAvailableAssetSize,
			CandidateAssets:      append([]string(nil), c.probeCandidates...),
		}, nil
	}
	if !c.found || c.release == nil {
		return probeResult{
			Status:               probeStatusNoCandidate,
			LatestVersion:        latest,
			InstallableVersion:   installable,
			LatestInstallable:    c.probeLatestInstallable,
			ExpectedPattern:      expectedPattern,
			AvailableAssetsCount: c.probeAvailableAssetSize,
			CandidateAssets:      append([]string(nil), c.probeCandidates...),
		}, nil
	}

	return probeResult{
		Status:               probeStatusMatched,
		Release:              c.release,
		LatestVersion:        latest,
		InstallableVersion:   installable,
		LatestInstallable:    c.probeLatestInstallable || latest == installable,
		ExpectedPattern:      expectedPattern,
		AvailableAssetsCount: c.probeAvailableAssetSize,
		CandidateAssets:      append([]string(nil), c.probeCandidates...),
	}, nil
}

func (c *fakeClient) UpdateTo(_ context.Context, rel releaseView, cmdPath string) error {
	_ = rel
	c.updateCalls++
	c.lastUpdatePath = cmdPath
	return c.updateErr
}

type stubSource struct {
	releases []selfupdate.SourceRelease
	listErr  error
}

func (s stubSource) ListReleases(context.Context, selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.releases, nil
}

func (s stubSource) DownloadReleaseAsset(context.Context, *selfupdate.Release, int64) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

type stubSourceRelease struct {
	id         int64
	tagName    string
	draft      bool
	prerelease bool
	assets     []selfupdate.SourceAsset
}

func (r stubSourceRelease) GetID() int64              { return r.id }
func (r stubSourceRelease) GetTagName() string        { return r.tagName }
func (r stubSourceRelease) GetDraft() bool            { return r.draft }
func (r stubSourceRelease) GetPrerelease() bool       { return r.prerelease }
func (r stubSourceRelease) GetPublishedAt() time.Time { return time.Now() }
func (r stubSourceRelease) GetReleaseNotes() string   { return "" }
func (r stubSourceRelease) GetName() string           { return r.tagName }
func (r stubSourceRelease) GetURL() string            { return "https://example.com/release" }
func (r stubSourceRelease) GetAssets() []selfupdate.SourceAsset {
	return r.assets
}

type stubSourceAsset struct {
	id   int64
	name string
	size int
}

func (a stubSourceAsset) GetID() int64                  { return a.id }
func (a stubSourceAsset) GetName() string               { return a.name }
func (a stubSourceAsset) GetSize() int                  { return a.size }
func (a stubSourceAsset) GetBrowserDownloadURL() string { return "https://example.com/asset" }

type blankSourceAsset struct{}

func (blankSourceAsset) GetID() int64                  { return 0 }
func (blankSourceAsset) GetName() string               { return " " }
func (blankSourceAsset) GetSize() int                  { return 0 }
func (blankSourceAsset) GetBrowserDownloadURL() string { return " " }

func TestResolveAssetTarget(t *testing.T) {
	tests := []struct {
		name         string
		goos         string
		goarch       string
		wantOS       string
		wantArch     string
		wantExt      string
		wantAsset    string
		expectErrMsg string
	}{
		{
			name:      "linux amd64",
			goos:      "linux",
			goarch:    "amd64",
			wantOS:    "linux",
			wantArch:  "x86_64",
			wantExt:   "tar.gz",
			wantAsset: "neocode_linux_x86_64.tar.gz",
		},
		{
			name:      "darwin arm64",
			goos:      "darwin",
			goarch:    "arm64",
			wantOS:    "darwin",
			wantArch:  "arm64",
			wantExt:   "tar.gz",
			wantAsset: "neocode_darwin_arm64.tar.gz",
		},
		{
			name:      "windows amd64",
			goos:      "windows",
			goarch:    "amd64",
			wantOS:    "windows",
			wantArch:  "x86_64",
			wantExt:   "zip",
			wantAsset: "neocode_windows_x86_64.zip",
		},
		{
			name:         "unsupported os",
			goos:         "freebsd",
			goarch:       "amd64",
			expectErrMsg: "unsupported os",
		},
		{
			name:         "unsupported arch",
			goos:         "linux",
			goarch:       "386",
			expectErrMsg: "unsupported arch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := resolveAssetTarget(tt.goos, tt.goarch)
			if tt.expectErrMsg != "" {
				if err == nil || !regexp.MustCompile(tt.expectErrMsg).MatchString(err.Error()) {
					t.Fatalf("resolveAssetTarget() error = %v, want contains %q", err, tt.expectErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveAssetTarget() error = %v", err)
			}
			if target.OSToken != tt.wantOS {
				t.Fatalf("OSToken = %q, want %q", target.OSToken, tt.wantOS)
			}
			if target.ArchToken != tt.wantArch {
				t.Fatalf("ArchToken = %q, want %q", target.ArchToken, tt.wantArch)
			}
			if target.Ext != tt.wantExt {
				t.Fatalf("Ext = %q, want %q", target.Ext, tt.wantExt)
			}
			if target.AssetName != tt.wantAsset {
				t.Fatalf("AssetName = %q, want %q", target.AssetName, tt.wantAsset)
			}
		})
	}
}

func TestBuildSelfupdateConfigUsesSemanticConfigAndChecksum(t *testing.T) {
	target := assetTarget{
		OSToken:   "darwin",
		ArchToken: "x86_64",
		Ext:       "tar.gz",
		AssetName: "neocode_darwin_x86_64.tar.gz",
	}
	config := buildSelfupdateConfig(target, true)
	if config.OS != "darwin" || config.Arch != "x86_64" {
		t.Fatalf("OS/Arch = %q/%q, want %q/%q", config.OS, config.Arch, "darwin", "x86_64")
	}
	if !config.Prerelease {
		t.Fatal("expected prerelease to be enabled")
	}
	if len(config.Filters) != 0 {
		t.Fatalf("len(Filters) = %d, want 0", len(config.Filters))
	}
	validator, ok := config.Validator.(*selfupdate.ChecksumValidator)
	if !ok {
		t.Fatalf("validator type = %T, want *selfupdate.ChecksumValidator", config.Validator)
	}
	if validator.UniqueFilename != checksumFilename {
		t.Fatalf("UniqueFilename = %q, want %q", validator.UniqueFilename, checksumFilename)
	}
}

func TestCheckLatest(t *testing.T) {
	originalNewClient := newClient
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "linux"
	runtimeGOARCH = "amd64"

	client := &fakeClient{
		release: fakeRelease{
			version: "v1.2.0",
			greaterFn: func(other string) bool {
				return other == "v1.1.0"
			},
		},
		found: true,
	}
	newClient = func(config selfupdate.Config) (updateClient, error) {
		return client, nil
	}

	result, err := CheckLatest(context.Background(), CheckOptions{
		CurrentVersion:    "v1.1.0",
		IncludePrerelease: false,
	})
	if err != nil {
		t.Fatalf("CheckLatest() error = %v", err)
	}
	if !result.HasUpdate {
		t.Fatal("expected HasUpdate to be true")
	}
	if !result.ComparableLatest {
		t.Fatal("expected ComparableLatest to be true")
	}
	if result.LatestVersion != "v1.2.0" {
		t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v1.2.0")
	}
	if result.InstallableVersion != "v1.2.0" {
		t.Fatalf("InstallableVersion = %q, want %q", result.InstallableVersion, "v1.2.0")
	}
}

func TestCheckLatestErrorBranches(t *testing.T) {
	originalNewClient := newClient
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})

	t.Run("unsupported platform", func(t *testing.T) {
		runtimeGOOS = "plan9"
		runtimeGOARCH = "amd64"

		result, err := CheckLatest(context.Background(), CheckOptions{CurrentVersion: ""})
		if err == nil || !regexp.MustCompile(`unsupported os`).MatchString(err.Error()) {
			t.Fatalf("CheckLatest() error = %v, want unsupported os", err)
		}
		if result.CurrentVersion != "dev" {
			t.Fatalf("CurrentVersion = %q, want %q", result.CurrentVersion, "dev")
		}
	})

	t.Run("new client failure", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return nil, errors.New("new client failed")
		}

		_, err := CheckLatest(context.Background(), CheckOptions{CurrentVersion: "v1.0.0"})
		if err == nil || err.Error() != "new client failed" {
			t.Fatalf("CheckLatest() error = %v, want new client failed", err)
		}
	})

	t.Run("detect latest failure", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return &fakeClient{detectErr: errors.New("detect failed")}, nil
		}

		_, err := CheckLatest(context.Background(), CheckOptions{CurrentVersion: "v1.0.0"})
		if err == nil || err.Error() != "detect failed" {
			t.Fatalf("CheckLatest() error = %v, want detect failed", err)
		}
	})

	t.Run("not found release", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return &fakeClient{found: false}, nil
		}

		result, err := CheckLatest(context.Background(), CheckOptions{CurrentVersion: "  v1.0.0  "})
		if err != nil {
			t.Fatalf("CheckLatest() error = %v", err)
		}
		if result.CurrentVersion != "v1.0.0" {
			t.Fatalf("CurrentVersion = %q, want %q", result.CurrentVersion, "v1.0.0")
		}
		if result.HasUpdate {
			t.Fatalf("HasUpdate = true, want false")
		}
		if result.ComparableLatest {
			t.Fatalf("ComparableLatest = true, want false")
		}
	})

	t.Run("empty latest version", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return &fakeClient{
				release: fakeRelease{version: "   "},
				found:   true,
			}, nil
		}

		result, err := CheckLatest(context.Background(), CheckOptions{CurrentVersion: "v1.0.0"})
		if err != nil {
			t.Fatalf("CheckLatest() error = %v", err)
		}
		if result.LatestVersion != "" || result.HasUpdate {
			t.Fatalf("unexpected result: %+v", result)
		}
		if result.ComparableLatest {
			t.Fatalf("ComparableLatest = true, want false")
		}
	})

	t.Run("non semver current version never marks update", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return &fakeClient{
				release: fakeRelease{
					version: "v9.9.9",
					greaterFn: func(string) bool {
						return true
					},
				},
				found: true,
			}, nil
		}

		result, err := CheckLatest(context.Background(), CheckOptions{CurrentVersion: "dev"})
		if err != nil {
			t.Fatalf("CheckLatest() error = %v", err)
		}
		if result.HasUpdate {
			t.Fatalf("HasUpdate = true, want false for non-semver current version")
		}
		if !result.ComparableLatest {
			t.Fatalf("ComparableLatest = false, want true when release is installable")
		}
	})

	t.Run("latest version exists but current platform not installable", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return &fakeClient{
				probeStatus:             probeStatusMatched,
				probeLatestVersion:      "v2.0.0",
				probeInstallableVersion: "v1.9.0",
				release: fakeRelease{
					version: "v1.9.0",
					greaterFn: func(other string) bool {
						return other == "v1.0.0"
					},
				},
				found: true,
			}, nil
		}

		result, err := CheckLatest(context.Background(), CheckOptions{CurrentVersion: "v1.0.0"})
		if err != nil {
			t.Fatalf("CheckLatest() error = %v", err)
		}
		if result.LatestVersion != "v2.0.0" {
			t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v2.0.0")
		}
		if result.InstallableVersion != "v1.9.0" {
			t.Fatalf("InstallableVersion = %q, want %q", result.InstallableVersion, "v1.9.0")
		}
		if !result.HasUpdate {
			t.Fatalf("HasUpdate = false, want true when installable version is newer")
		}
		if result.ComparableLatest {
			t.Fatalf("ComparableLatest = true, want false when latest is not installable")
		}
	})
}

func TestDoUpdateSkipsWhenAlreadyLatestForSemver(t *testing.T) {
	originalNewClient := newClient
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "linux"
	runtimeGOARCH = "amd64"

	client := &fakeClient{
		release: fakeRelease{
			version: "v1.2.0",
			greaterFn: func(other string) bool {
				return false
			},
		},
		found: true,
	}
	newClient = func(config selfupdate.Config) (updateClient, error) {
		return client, nil
	}

	result, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.2.0"})
	if err != nil {
		t.Fatalf("DoUpdate() error = %v", err)
	}
	if result.Updated {
		t.Fatal("expected Updated to be false")
	}
	if client.updateCalls != 0 {
		t.Fatalf("update calls = %d, want 0", client.updateCalls)
	}
}

func TestDoUpdateUsesUpdaterLibraryPathForWindows(t *testing.T) {
	originalNewClient := newClient
	originalExePath := resolveExecutablePath
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		resolveExecutablePath = originalExePath
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "windows"
	runtimeGOARCH = "amd64"

	client := &fakeClient{
		release: fakeRelease{
			version: "v1.3.0",
			greaterFn: func(other string) bool {
				return false
			},
		},
		found: true,
	}

	var capturedConfig selfupdate.Config
	newClient = func(config selfupdate.Config) (updateClient, error) {
		capturedConfig = config
		return client, nil
	}
	resolveExecutablePath = func() (string, error) {
		return `C:\Tools\neocode.exe`, nil
	}

	result, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "dev"})
	if err != nil {
		t.Fatalf("DoUpdate() error = %v", err)
	}
	if !result.Updated {
		t.Fatal("expected Updated to be true")
	}
	if client.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", client.updateCalls)
	}
	if client.lastUpdatePath != `C:\Tools\neocode.exe` {
		t.Fatalf("last update path = %q, want %q", client.lastUpdatePath, `C:\Tools\neocode.exe`)
	}
	if capturedConfig.OS != "windows" || capturedConfig.Arch != "x86_64" {
		t.Fatalf("config OS/Arch = %q/%q, want %q/%q", capturedConfig.OS, capturedConfig.Arch, "windows", "x86_64")
	}
}

func TestDoUpdatePropagatesUpdateError(t *testing.T) {
	originalNewClient := newClient
	originalExePath := resolveExecutablePath
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		resolveExecutablePath = originalExePath
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "linux"
	runtimeGOARCH = "amd64"

	expected := errors.New("apply update failed")
	client := &fakeClient{
		release: fakeRelease{
			version: "v1.3.0",
			greaterFn: func(other string) bool {
				return true
			},
		},
		found:     true,
		updateErr: expected,
	}

	newClient = func(config selfupdate.Config) (updateClient, error) {
		return client, nil
	}
	resolveExecutablePath = func() (string, error) {
		return "/usr/local/bin/neocode", nil
	}

	_, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.2.0"})
	if !errors.Is(err, expected) {
		t.Fatalf("DoUpdate() error = %v, want %v", err, expected)
	}
}

func TestDoUpdateErrorAndEdgeBranches(t *testing.T) {
	originalNewClient := newClient
	originalExePath := resolveExecutablePath
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		resolveExecutablePath = originalExePath
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})

	t.Run("unsupported platform", func(t *testing.T) {
		runtimeGOOS = "plan9"
		runtimeGOARCH = "amd64"

		result, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: ""})
		if err == nil || !regexp.MustCompile(`unsupported os`).MatchString(err.Error()) {
			t.Fatalf("DoUpdate() error = %v, want unsupported os", err)
		}
		if result.CurrentVersion != "dev" {
			t.Fatalf("CurrentVersion = %q, want %q", result.CurrentVersion, "dev")
		}
	})

	t.Run("new client failure", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return nil, errors.New("new client failed")
		}

		_, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.0.0"})
		if err == nil || err.Error() != "new client failed" {
			t.Fatalf("DoUpdate() error = %v, want new client failed", err)
		}
	})

	t.Run("detect latest failure", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return &fakeClient{detectErr: errors.New("detect failed")}, nil
		}

		_, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.0.0"})
		if err == nil || err.Error() != "detect failed" {
			t.Fatalf("DoUpdate() error = %v, want detect failed", err)
		}
	})

	t.Run("release not found", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return &fakeClient{found: false}, nil
		}

		_, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.0.0"})
		if err == nil || !regexp.MustCompile(`no release asset found`).MatchString(err.Error()) {
			t.Fatalf("DoUpdate() error = %v, want no release asset found", err)
		}
	})

	t.Run("resolve executable path failure", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"
		newClient = func(selfupdate.Config) (updateClient, error) {
			return &fakeClient{
				release: fakeRelease{
					version: "v1.3.0",
					greaterFn: func(string) bool {
						return true
					},
				},
				found: true,
			}, nil
		}
		resolveExecutablePath = func() (string, error) {
			return "", errors.New("resolve exec failed")
		}

		_, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.2.0"})
		if err == nil || err.Error() != "resolve exec failed" {
			t.Fatalf("DoUpdate() error = %v, want resolve exec failed", err)
		}
	})

	t.Run("dev version updates without semver compare", func(t *testing.T) {
		runtimeGOOS = "linux"
		runtimeGOARCH = "amd64"

		client := &fakeClient{
			release: fakeRelease{
				version: "v1.3.0",
				greaterFn: func(string) bool {
					return false
				},
			},
			found: true,
		}
		newClient = func(selfupdate.Config) (updateClient, error) {
			return client, nil
		}
		resolveExecutablePath = func() (string, error) {
			return "/tmp/neocode", nil
		}

		result, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "dev"})
		if err != nil {
			t.Fatalf("DoUpdate() error = %v", err)
		}
		if !result.Updated {
			t.Fatalf("Updated = false, want true")
		}
		if client.updateCalls != 1 {
			t.Fatalf("update calls = %d, want 1", client.updateCalls)
		}
	})
}

func TestDoUpdateReturnsDiagnosticWhenNoCandidate(t *testing.T) {
	originalNewClient := newClient
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "windows"
	runtimeGOARCH = "amd64"

	newClient = func(selfupdate.Config) (updateClient, error) {
		return &fakeClient{
			found:                   false,
			probeLatestVersion:      "v1.4.0",
			probeAvailableAssetSize: 3,
			probeCandidates: []string{
				"neocode_Windows_x86_64.zip",
				"checksums.txt",
				"neocode_Darwin_arm64.tar.gz",
			},
		}, nil
	}

	_, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.3.0"})
	if err == nil {
		t.Fatal("expected diagnostic error")
	}
	text := err.Error()
	if !strings.Contains(text, "no release asset found for current platform") {
		t.Fatalf("error = %v, want no release asset found", err)
	}
	if !strings.Contains(text, "os=windows") || !strings.Contains(text, "arch=x86_64") {
		t.Fatalf("error = %v, want os/arch fields", err)
	}
	if !strings.Contains(text, "expected-pattern=") || !strings.Contains(text, "available-assets-count=3") {
		t.Fatalf("error = %v, want diagnostic fields", err)
	}
	if !strings.Contains(text, "candidate-assets=[neocode_Windows_x86_64.zip checksums.txt") {
		t.Fatalf("error = %v, want candidate assets", err)
	}
}

func TestDoUpdateReturnsDiagnosticWhenAmbiguous(t *testing.T) {
	originalNewClient := newClient
	originalGOOS := runtimeGOOS
	originalGOARCH := runtimeGOARCH
	t.Cleanup(func() {
		newClient = originalNewClient
		runtimeGOOS = originalGOOS
		runtimeGOARCH = originalGOARCH
	})
	runtimeGOOS = "linux"
	runtimeGOARCH = "amd64"

	newClient = func(selfupdate.Config) (updateClient, error) {
		return &fakeClient{
			release: fakeRelease{
				version: "v1.4.0",
			},
			probeStatus:             probeStatusAmbiguous,
			probeLatestVersion:      "v1.4.0",
			probeAvailableAssetSize: 4,
			probeCandidates: []string{
				"neocode_linux_x86_64.tar.gz",
				"neocode-linux-amd64.tgz",
			},
		}, nil
	}

	_, err := DoUpdate(context.Background(), UpdateOptions{CurrentVersion: "v1.3.0"})
	if err == nil {
		t.Fatal("expected diagnostic error")
	}
	text := err.Error()
	if !strings.Contains(text, "multiple release assets matched current platform") {
		t.Fatalf("error = %v, want ambiguous message", err)
	}
	if !strings.Contains(text, "available-assets-count=4") {
		t.Fatalf("error = %v, want available assets count", err)
	}
	if !strings.Contains(text, "candidate-assets=[neocode_linux_x86_64.tar.gz neocode-linux-amd64.tgz]") {
		t.Fatalf("error = %v, want candidate assets", err)
	}
}

func TestBuildExpectedPatternMatchesNamingVariants(t *testing.T) {
	target := assetTarget{
		OSToken:   "windows",
		ArchToken: "x86_64",
		Ext:       "zip",
	}
	matcher := regexp.MustCompile(buildExpectedPattern(target))
	cases := []struct {
		name  string
		asset string
		match bool
	}{
		{name: "underscore", asset: "neocode_Windows_x86_64.zip", match: true},
		{name: "hyphen amd64", asset: "neocode-windows-amd64.zip", match: true},
		{name: "extra suffix", asset: "neocode_win_x86-64_rc1.zip", match: true},
		{name: "invalid arch", asset: "neocode_windows_arm64.zip", match: false},
		{name: "invalid ext", asset: "neocode_windows_x86_64.tar.gz", match: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := matcher.MatchString(strings.ToLower(tt.asset))
			if got != tt.match {
				t.Fatalf("MatchString(%q) = %v, want %v", tt.asset, got, tt.match)
			}
		})
	}
}

func TestSampleAssetsForDiagnosticTruncatesByCountAndLength(t *testing.T) {
	longName := "neocode_windows_x86_64_" + strings.Repeat("a", 200) + ".zip"
	input := make([]string, 0, 12)
	input = append(input, longName)
	for i := 0; i < 11; i++ {
		input = append(input, fmt.Sprintf("asset-%02d", i))
	}

	sampled := sampleAssetsForDiagnostic(input)
	if len(sampled) != maxDiagnosticCandidateAssets {
		t.Fatalf("len(sampled) = %d, want %d", len(sampled), maxDiagnosticCandidateAssets)
	}
	if !strings.HasSuffix(sampled[0], "...") {
		t.Fatalf("sampled[0] = %q, want truncated suffix", sampled[0])
	}
}

func TestSelfupdateClientProbeLatestForNamingVariantsAndAmbiguity(t *testing.T) {
	t.Run("match naming variant", func(t *testing.T) {
		source := stubSource{
			releases: []selfupdate.SourceRelease{
				stubSourceRelease{
					id:      1,
					tagName: "v1.5.0",
					assets: []selfupdate.SourceAsset{
						stubSourceAsset{id: 1, name: "neocode-Windows-amd64.zip", size: 1},
					},
				},
			},
		}
		updater, err := selfupdate.NewUpdater(selfupdate.Config{
			Source: source,
			OS:     "windows",
			Arch:   "x86_64",
		})
		if err != nil {
			t.Fatalf("NewUpdater() error = %v", err)
		}

		client := selfupdateClient{
			updater: updater,
			source:  source,
			config: selfupdate.Config{
				Source: source,
				OS:     "windows",
				Arch:   "x86_64",
			},
		}
		target := assetTarget{
			OSToken:   "windows",
			ArchToken: "x86_64",
			Ext:       "zip",
		}
		probe, err := client.ProbeLatest(context.Background(), selfupdate.NewRepositorySlug(repositoryOwner, repositoryName), target)
		if err != nil {
			t.Fatalf("ProbeLatest() error = %v", err)
		}
		if probe.Status != probeStatusMatched {
			t.Fatalf("probe status = %v, want matched", probe.Status)
		}
		if probe.Release == nil {
			t.Fatal("expected probe release")
		}
		if probe.LatestVersion == "" {
			t.Fatal("expected latest version")
		}
	})

	t.Run("ambiguous assets", func(t *testing.T) {
		source := stubSource{
			releases: []selfupdate.SourceRelease{
				stubSourceRelease{
					id:      1,
					tagName: "v1.5.0",
					assets: []selfupdate.SourceAsset{
						stubSourceAsset{id: 1, name: "neocode_windows_x86_64.zip", size: 1},
						stubSourceAsset{id: 2, name: "neocode-windows-amd64.zip", size: 1},
					},
				},
			},
		}
		updater, err := selfupdate.NewUpdater(selfupdate.Config{
			Source: source,
			OS:     "windows",
			Arch:   "x86_64",
		})
		if err != nil {
			t.Fatalf("NewUpdater() error = %v", err)
		}

		client := selfupdateClient{
			updater: updater,
			source:  source,
			config: selfupdate.Config{
				Source: source,
				OS:     "windows",
				Arch:   "x86_64",
			},
		}
		target := assetTarget{
			OSToken:   "windows",
			ArchToken: "x86_64",
			Ext:       "zip",
		}
		probe, err := client.ProbeLatest(context.Background(), selfupdate.NewRepositorySlug(repositoryOwner, repositoryName), target)
		if err != nil {
			t.Fatalf("ProbeLatest() error = %v", err)
		}
		if probe.Status != probeStatusAmbiguous {
			t.Fatalf("probe status = %v, want ambiguous", probe.Status)
		}
		if len(probe.CandidateAssets) != 2 {
			t.Fatalf("len(candidate assets) = %d, want 2", len(probe.CandidateAssets))
		}
	})
}

func TestSelfupdateClientUpdateToUnsupportedType(t *testing.T) {
	target := assetTarget{
		OSToken:   "linux",
		ArchToken: "amd64",
		Ext:       "tar.gz",
		AssetName: "neocode_linux_amd64.tar.gz",
	}

	source := stubSource{
		releases: []selfupdate.SourceRelease{
			stubSourceRelease{
				id:      1,
				tagName: "v1.5.0",
				assets: []selfupdate.SourceAsset{
					stubSourceAsset{id: 1, name: target.AssetName, size: 1},
				},
			},
		},
	}
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
		OS:     target.OSToken,
		Arch:   target.ArchToken,
	})
	if err != nil {
		t.Fatalf("NewUpdater() error = %v", err)
	}

	client := selfupdateClient{updater: updater}
	release, found, err := updater.DetectVersion(
		context.Background(),
		selfupdate.NewRepositorySlug(repositoryOwner, repositoryName),
		"v1.5.0",
	)
	if err != nil {
		t.Fatalf("DetectVersion() error = %v", err)
	}
	if !found || release == nil {
		t.Fatalf("expected release found, got found=%v release=%v", found, release)
	}
	rel := selfupdateRelease{release: release}

	err = client.UpdateTo(context.Background(), fakeRelease{version: "v1.0.0"}, "/tmp/neocode")
	if err == nil || err.Error() != "updater: unsupported release type" {
		t.Fatalf("UpdateTo() error = %v, want unsupported release type", err)
	}

	err = client.UpdateTo(context.Background(), selfupdateRelease{}, "/tmp/neocode")
	if err == nil || err.Error() != "updater: unsupported release type" {
		t.Fatalf("UpdateTo() error = %v, want unsupported release type for nil release", err)
	}

	if err := client.UpdateTo(context.Background(), rel, "/tmp/neocode"); err == nil {
		t.Fatalf("expected UpdateTo() to fail with stub asset payload")
	}
}

func TestNewClientFactory(t *testing.T) {
	_, err := newClient(selfupdate.Config{Filters: []string{"("}})
	if err == nil {
		t.Fatalf("expected newClient to fail with invalid filter regex")
	}

	client, err := newClient(selfupdate.Config{
		Source: stubSource{},
		OS:     "linux",
		Arch:   "amd64",
	})
	if err != nil {
		t.Fatalf("newClient() unexpected error: %v", err)
	}
	if client == nil {
		t.Fatalf("expected non-nil client")
	}
}

func TestParseReleaseVersionBranches(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		ok   bool
	}{
		{name: "empty", tag: " ", ok: false},
		{name: "no semver", tag: "release-latest", ok: false},
		{name: "with v prefix", tag: "v1.2.3", ok: true},
		{name: "with uppercase v prefix", tag: "V1.2.3", ok: false},
		{name: "embedded semver text should be rejected", tag: "release/v1.2.3", ok: false},
		{name: "prerelease build", tag: "v1.2.3-rc.1+build.7", ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, ok := parseReleaseVersion(tt.tag)
			if ok != tt.ok {
				t.Fatalf("parseReleaseVersion(%q) ok = %v, want %v", tt.tag, ok, tt.ok)
			}
			if ok && ver == nil {
				t.Fatalf("parseReleaseVersion(%q) returned nil version", tt.tag)
			}
		})
	}
}

func TestAliasPatternHelpers(t *testing.T) {
	if got := platformAliasPattern("windows"); got != `(?:windows|win)` {
		t.Fatalf("platformAliasPattern(windows) = %q", got)
	}
	if got := platformAliasPattern("darwin"); got != `(?:darwin|macos)` {
		t.Fatalf("platformAliasPattern(darwin) = %q", got)
	}
	if got := platformAliasPattern(" Linux "); got != `linux` {
		t.Fatalf("platformAliasPattern(linux) = %q, want linux", got)
	}

	if got := archAliasPattern("x86_64"); got != `(?:x86_64|x86-64|amd64)` {
		t.Fatalf("archAliasPattern(x86_64) = %q", got)
	}
	if got := archAliasPattern("arm64"); got != `(?:arm64|aarch64)` {
		t.Fatalf("archAliasPattern(arm64) = %q", got)
	}
	if got := archAliasPattern(" riscv64 "); got != `riscv64` {
		t.Fatalf("archAliasPattern(riscv64) = %q, want riscv64", got)
	}
}

func TestBuildReleaseSnapshotFilters(t *testing.T) {
	matcher := regexp.MustCompile(`^neocode[-_]linux[-_](?:x86_64|amd64)(?:\.tar\.gz|\.tgz)$`)

	if _, ok := buildReleaseSnapshot(nil, false, matcher); ok {
		t.Fatal("expected nil release to be filtered")
	}

	draft := stubSourceRelease{draft: true, tagName: "v1.2.3"}
	if _, ok := buildReleaseSnapshot(draft, false, matcher); ok {
		t.Fatal("expected draft release to be filtered")
	}

	pre := stubSourceRelease{prerelease: true, tagName: "v1.2.3"}
	if _, ok := buildReleaseSnapshot(pre, false, matcher); ok {
		t.Fatal("expected prerelease to be filtered when includePrerelease=false")
	}

	invalidTag := stubSourceRelease{tagName: "latest"}
	if _, ok := buildReleaseSnapshot(invalidTag, true, matcher); ok {
		t.Fatal("expected non-semver tag to be filtered")
	}

	release := stubSourceRelease{
		tagName: "v1.2.3",
		assets: []selfupdate.SourceAsset{
			stubSourceAsset{name: "neocode_linux_amd64.tar.gz"},
			stubSourceAsset{name: "checksums.txt"},
		},
	}
	snapshot, ok := buildReleaseSnapshot(release, true, matcher)
	if !ok || snapshot == nil {
		t.Fatal("expected valid snapshot")
	}
	if len(snapshot.MatchedAssets) != 1 {
		t.Fatalf("len(MatchedAssets) = %d, want 1", len(snapshot.MatchedAssets))
	}
}

func TestSelfupdateClientProbeLatestNoMatchedAssetReturnsEligibleDiagnostic(t *testing.T) {
	source := stubSource{
		releases: []selfupdate.SourceRelease{
			stubSourceRelease{
				id:      1,
				tagName: "v1.8.0",
				assets: []selfupdate.SourceAsset{
					stubSourceAsset{id: 1, name: "checksums.txt", size: 1},
					stubSourceAsset{id: 2, name: "readme.txt", size: 1},
				},
			},
		},
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
		OS:     "linux",
		Arch:   "x86_64",
	})
	if err != nil {
		t.Fatalf("NewUpdater() error = %v", err)
	}

	client := selfupdateClient{
		updater: updater,
		source:  source,
		config: selfupdate.Config{
			Source: source,
			OS:     "linux",
			Arch:   "x86_64",
		},
	}
	target := assetTarget{
		OSToken:   "linux",
		ArchToken: "x86_64",
		Ext:       "tar.gz",
	}

	probe, err := client.ProbeLatest(context.Background(), selfupdate.NewRepositorySlug(repositoryOwner, repositoryName), target)
	if err != nil {
		t.Fatalf("ProbeLatest() error = %v", err)
	}
	if probe.Status != probeStatusNoCandidate {
		t.Fatalf("Status = %v, want no-candidate", probe.Status)
	}
	if probe.LatestVersion != "1.8.0" {
		t.Fatalf("LatestVersion = %q, want %q", probe.LatestVersion, "1.8.0")
	}
	if probe.AvailableAssetsCount != 2 {
		t.Fatalf("AvailableAssetsCount = %d, want 2", probe.AvailableAssetsCount)
	}
	if len(probe.CandidateAssets) != 2 {
		t.Fatalf("len(CandidateAssets) = %d, want 2", len(probe.CandidateAssets))
	}
}

func TestSelfupdateClientProbeLatestKeepsEligibleLatestAndInstallableLatest(t *testing.T) {
	source := stubSource{
		releases: []selfupdate.SourceRelease{
			stubSourceRelease{
				id:      2,
				tagName: "v2.0.0",
				assets: []selfupdate.SourceAsset{
					stubSourceAsset{id: 20, name: "checksums.txt", size: 1},
				},
			},
			stubSourceRelease{
				id:      1,
				tagName: "v1.9.0",
				assets: []selfupdate.SourceAsset{
					stubSourceAsset{id: 10, name: "neocode_linux_x86_64.tar.gz", size: 1},
					stubSourceAsset{id: 11, name: "checksums.txt", size: 1},
				},
			},
		},
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
		OS:     "linux",
		Arch:   "x86_64",
	})
	if err != nil {
		t.Fatalf("NewUpdater() error = %v", err)
	}

	client := selfupdateClient{
		updater: updater,
		source:  source,
		config: selfupdate.Config{
			Source: source,
			OS:     "linux",
			Arch:   "x86_64",
		},
	}
	target := assetTarget{
		OSToken:   "linux",
		ArchToken: "x86_64",
		Ext:       "tar.gz",
	}

	probe, err := client.ProbeLatest(context.Background(), selfupdate.NewRepositorySlug(repositoryOwner, repositoryName), target)
	if err != nil {
		t.Fatalf("ProbeLatest() error = %v", err)
	}
	if probe.Status != probeStatusMatched {
		t.Fatalf("Status = %v, want matched", probe.Status)
	}
	if probe.LatestVersion != "2.0.0" {
		t.Fatalf("LatestVersion = %q, want %q", probe.LatestVersion, "2.0.0")
	}
	if probe.InstallableVersion != "1.9.0" {
		t.Fatalf("InstallableVersion = %q, want %q", probe.InstallableVersion, "1.9.0")
	}
	if probe.LatestInstallable {
		t.Fatalf("LatestInstallable = true, want false")
	}
}

func TestSelfupdateClientProbeLatestListReleasesError(t *testing.T) {
	client := selfupdateClient{
		source: stubSource{listErr: errors.New("list failed")},
		config: selfupdate.Config{OS: "linux", Arch: "x86_64"},
	}

	target := assetTarget{OSToken: "linux", ArchToken: "x86_64", Ext: "tar.gz"}
	_, err := client.ProbeLatest(context.Background(), selfupdate.NewRepositorySlug(repositoryOwner, repositoryName), target)
	if err == nil || err.Error() != "list failed" {
		t.Fatalf("ProbeLatest() error = %v, want list failed", err)
	}
}

func TestDetectReleaseByTagAndAssetBranches(t *testing.T) {
	source := stubSource{
		releases: []selfupdate.SourceRelease{
			stubSourceRelease{
				id:      1,
				tagName: "v1.0.0",
				assets: []selfupdate.SourceAsset{
					stubSourceAsset{id: 1, name: "neocode_linux_x86_64.tar.gz", size: 1},
				},
			},
		},
	}
	client := selfupdateClient{
		source: source,
		config: selfupdate.Config{
			Source: source,
			OS:     "linux",
			Arch:   "x86_64",
		},
	}

	repository := selfupdate.NewRepositorySlug(repositoryOwner, repositoryName)
	target := assetTarget{OSToken: "linux", ArchToken: "x86_64", Ext: "tar.gz"}

	rel, found, err := client.detectReleaseByTagAndAsset(context.Background(), repository, " ", "asset", target)
	if err != nil || found || rel != nil {
		t.Fatalf("empty tag result = (%v, %v, %v), want (nil, false, nil)", rel, found, err)
	}

	rel, found, err = client.detectReleaseByTagAndAsset(context.Background(), repository, "v1.0.0", " ", target)
	if err != nil || found || rel != nil {
		t.Fatalf("empty asset result = (%v, %v, %v), want (nil, false, nil)", rel, found, err)
	}

	errClient := selfupdateClient{
		source: stubSource{listErr: errors.New("list failed")},
		config: selfupdate.Config{
			Source: stubSource{listErr: errors.New("list failed")},
			OS:     "linux",
			Arch:   "x86_64",
		},
	}
	_, _, err = errClient.detectReleaseByTagAndAsset(
		context.Background(),
		repository,
		"v1.0.0",
		"neocode_linux_x86_64.tar.gz",
		target,
	)
	if err == nil || err.Error() != "list failed" {
		t.Fatalf("detectReleaseByTagAndAsset() error = %v, want list failed", err)
	}
}

func TestAssetDiagnosticHelperBranches(t *testing.T) {
	names := collectAssetNames([]selfupdate.SourceAsset{
		stubSourceAsset{name: "z-last"},
		blankSourceAsset{},
		stubSourceAsset{name: "a-first"},
	})
	if len(names) != 2 || names[0] != "a-first" || names[1] != "z-last" {
		t.Fatalf("collectAssetNames() = %v", names)
	}

	if got := trimDiagnosticAssetName(" "); got != "" {
		t.Fatalf("trimDiagnosticAssetName(blank) = %q, want empty", got)
	}
	if got := trimDiagnosticAssetName("bad-\x1b[31masset\x1b[0m-\nname"); got != "bad-asset-name" {
		t.Fatalf("trimDiagnosticAssetName(ansi) = %q, want %q", got, "bad-asset-name")
	}
	if got := sanitizeDiagnosticText("\x1b[31masset\x1b[0m\t\nok"); got != "assetok" {
		t.Fatalf("sanitizeDiagnosticText() = %q, want %q", got, "assetok")
	}
	if got := sanitizeDiagnosticAssets([]string{"\x1b[31mone\x1b[0m", "  ", "two"}); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Fatalf("sanitizeDiagnosticAssets() = %v, want [one two]", got)
	}

	if got := firstNonEmptyAssetName([]selfupdate.SourceAsset{blankSourceAsset{}}); got != "" {
		t.Fatalf("firstNonEmptyAssetName() = %q, want empty", got)
	}

	if got := assetName(nil); got != "" {
		t.Fatalf("assetName(nil) = %q, want empty", got)
	}
	if got := assetName(blankSourceAsset{}); got != "" {
		t.Fatalf("assetName(blankSourceAsset) = %q, want empty", got)
	}
}
