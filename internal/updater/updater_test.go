package updater

import (
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
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
	release        releaseView
	found          bool
	detectErr      error
	updateErr      error
	updateCalls    int
	lastUpdatePath string
}

func (c *fakeClient) DetectLatest(context.Context, selfupdate.Repository) (releaseView, bool, error) {
	return c.release, c.found, c.detectErr
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
			wantOS:    "Linux",
			wantArch:  "x86_64",
			wantExt:   "tar.gz",
			wantAsset: "neocode_Linux_x86_64.tar.gz",
		},
		{
			name:      "darwin arm64",
			goos:      "darwin",
			goarch:    "arm64",
			wantOS:    "Darwin",
			wantArch:  "arm64",
			wantExt:   "tar.gz",
			wantAsset: "neocode_Darwin_arm64.tar.gz",
		},
		{
			name:      "windows amd64",
			goos:      "windows",
			goarch:    "amd64",
			wantOS:    "Windows",
			wantArch:  "x86_64",
			wantExt:   "zip",
			wantAsset: "neocode_Windows_x86_64.zip",
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

func TestBuildSelfupdateConfigUsesExactFilterAndChecksum(t *testing.T) {
	target := assetTarget{
		OSToken:   "Darwin",
		ArchToken: "x86_64",
		Ext:       "tar.gz",
		AssetName: "neocode_Darwin_x86_64.tar.gz",
	}
	config := buildSelfupdateConfig(target, true)
	if config.OS != "Darwin" || config.Arch != "x86_64" {
		t.Fatalf("OS/Arch = %q/%q, want %q/%q", config.OS, config.Arch, "Darwin", "x86_64")
	}
	if !config.Prerelease {
		t.Fatal("expected prerelease to be enabled")
	}
	if len(config.Filters) != 1 {
		t.Fatalf("len(Filters) = %d, want 1", len(config.Filters))
	}
	exactFilter := config.Filters[0]
	re := regexp.MustCompile(exactFilter)
	if !re.MatchString("neocode_Darwin_x86_64.tar.gz") {
		t.Fatal("exact filter should match target asset")
	}
	if re.MatchString("neocode_Darwin_x86_64.tar.gz.sig") {
		t.Fatal("exact filter should not match similar asset names")
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
	if result.LatestVersion != "v1.2.0" {
		t.Fatalf("LatestVersion = %q, want %q", result.LatestVersion, "v1.2.0")
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
	if capturedConfig.OS != "Windows" || capturedConfig.Arch != "x86_64" {
		t.Fatalf("config OS/Arch = %q/%q, want %q/%q", capturedConfig.OS, capturedConfig.Arch, "Windows", "x86_64")
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

func TestSelfupdateClientDetectLatestAndUnsupportedUpdateType(t *testing.T) {
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
	rel, found, err := client.DetectLatest(context.Background(), selfupdate.NewRepositorySlug(repositoryOwner, repositoryName))
	if err != nil {
		t.Fatalf("DetectLatest() error = %v", err)
	}
	if !found || rel == nil {
		t.Fatalf("expected release found, got found=%v rel=%v", found, rel)
	}
	if rel.Version() == "" {
		t.Fatalf("expected non-empty release version")
	}
	if !rel.GreaterThan("1.0.0") {
		t.Fatalf("expected release to be greater than 1.0.0")
	}

	noReleaseUpdater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: stubSource{releases: nil},
		OS:     target.OSToken,
		Arch:   target.ArchToken,
	})
	if err != nil {
		t.Fatalf("NewUpdater(no release) error = %v", err)
	}
	noReleaseClient := selfupdateClient{updater: noReleaseUpdater}
	if gotRel, gotFound, gotErr := noReleaseClient.DetectLatest(
		context.Background(),
		selfupdate.NewRepositorySlug(repositoryOwner, repositoryName),
	); gotErr != nil || gotFound || gotRel != nil {
		t.Fatalf("DetectLatest(no release) = (%v, %v, %v), want (nil, false, nil)", gotRel, gotFound, gotErr)
	}

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
