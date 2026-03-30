package cliutils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/hashicorp/go-version"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

const (
	// GitLab Duo CLI project: https://gitlab.com/gitlab-org/editor-extensions/gitlab-lsp
	duoProjectID            = "46519181"
	duoPackageName          = "duo-cli"
	defaultUpdateCheckDelay = 24 * time.Hour

	// duoMaxCompatibleMajorVersion is the maximum Duo CLI major version this build of glab
	// supports. Auto-updates are bounded to this major version. When Duo CLI releases a new
	// major version, glab must be updated to increment this value after validating compatibility.
	duoMaxCompatibleMajorVersion = 8
)

// BinaryInfo represents metadata about the installed Duo CLI binary.
type BinaryInfo struct {
	Path     string
	Version  string
	Checksum string
}

// packageAsset represents a Duo CLI package asset from GitLab.
type packageAsset struct {
	version  string
	filename string
	checksum string
}

// BinaryManager handles the lifecycle of the Duo CLI binary.
type BinaryManager struct {
	io     *iostreams.IOStreams
	client *gitlab.Client
}

// NewBinaryManager creates a new BinaryManager instance.
func NewBinaryManager(io *iostreams.IOStreams) *BinaryManager {
	client, _ := gitlab.NewAuthSourceClient(gitlab.Unauthenticated{})
	return &BinaryManager{
		io:     io,
		client: client,
	}
}

// EnsureInstalled ensures the Duo CLI binary is installed and returns metadata.
// If the binary is not installed, it prompts the user and downloads it.
// The caller should check if binaryPath matches expectedPath and if binary is valid.
func (m *BinaryManager) EnsureInstalled(ctx context.Context, installedVersion, installedPath string, autoDownload string) (*BinaryInfo, error) {
	platform, err := detectPlatform()
	if err != nil {
		return nil, err
	}

	binaryPath := platform.binaryPath()

	if installedVersion != "" && installedPath == binaryPath && m.isBinaryValid(binaryPath) {
		return &BinaryInfo{
			Path:     binaryPath,
			Version:  installedVersion,
			Checksum: "",
		}, nil
	}

	shouldDownload, _, err := m.promptDownload(ctx, autoDownload)
	if err != nil {
		return nil, err
	}
	if !shouldDownload {
		return nil, errors.New("download cancelled")
	}

	return m.downloadAndInstall(ctx, platform)
}

// CheckForUpdate checks if a newer version of Duo CLI is available within the supported major version.
// Returns (hasUpdate, latestVersion, newMajorVersion, newCheckTime, error).
// newMajorVersion is non-empty when the latest release has a higher major than supported.
// Caller should save newCheckTime to config if non-zero.
func (m *BinaryManager) CheckForUpdate(ctx context.Context, currentVersion string, lastCheckTime time.Time, forceCheck bool) (bool, string, string, time.Time, error) {
	if !forceCheck && !lastCheckTime.IsZero() && time.Since(lastCheckTime) < defaultUpdateCheckDelay {
		return false, "", "", time.Time{}, nil
	}

	latestPkg, err := m.fetchLatestPackage(ctx)
	if err != nil {
		return false, "", "", time.Time{}, err
	}

	newCheckTime := time.Now()

	latestV, err := version.NewVersion(latestPkg.Version)
	if err != nil {
		return false, "", "", newCheckTime, fmt.Errorf("invalid latest version: %w", err)
	}

	segments := latestV.Segments()
	if len(segments) == 0 {
		return false, "", "", newCheckTime, fmt.Errorf("invalid latest version: %q", latestPkg.Version)
	}
	if segments[0] > duoMaxCompatibleMajorVersion {
		return false, "", latestPkg.Version, newCheckTime, nil
	}

	current, err := version.NewVersion(currentVersion)
	if err != nil {
		return false, "", "", newCheckTime, fmt.Errorf("invalid current version: %w", err)
	}

	if latestV.GreaterThan(current) {
		return true, latestPkg.Version, "", newCheckTime, nil
	}

	return false, latestPkg.Version, "", newCheckTime, nil
}

// update downloads and installs the latest version of Duo CLI.
// Returns BinaryInfo for caller to save to config.
func (m *BinaryManager) Update(ctx context.Context) (*BinaryInfo, error) {
	platform, err := detectPlatform()
	if err != nil {
		return nil, err
	}

	return m.downloadAndInstall(ctx, platform)
}

// isBinaryValid checks if the binary exists and is executable.
func (m *BinaryManager) isBinaryValid(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if !info.Mode().IsRegular() {
		return false
	}

	// Unix only - Windows doesn't use execute bit
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return false
	}

	return true
}

// promptDownload prompts the user to download the Duo CLI binary.
// Returns (shouldDownload, savePreference, preferenceValue, error).
// Caller should save preference if savePreference is true.
func (m *BinaryManager) promptDownload(ctx context.Context, autoDownload string) (bool, string, error) {
	if autoDownload == "true" {
		return true, "", nil
	}

	// "false" means "don't auto-download", not "never download"
	var confirm bool
	if err := m.io.Confirm(ctx, &confirm, "Download GitLab Duo CLI binary?"); err != nil {
		return false, "", err
	}

	if !confirm {
		return false, "", errors.New("download cancelled")
	}

	var always bool
	if err := m.io.Confirm(ctx, &always, "Always download updates automatically?"); err != nil {
		return true, "", nil
	}

	if always {
		return true, "true", nil
	}

	return true, "", nil
}

// downloadAndInstall downloads the Duo CLI binary and installs it.
// Returns BinaryInfo for caller to save to config.
func (m *BinaryManager) downloadAndInstall(ctx context.Context, platform platform) (*BinaryInfo, error) {
	asset, err := m.fetchPackageAsset(ctx, platform)
	if err != nil {
		return nil, err
	}

	tempFile, err := os.CreateTemp("", "duo-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	m.io.LogInfof("Downloading %s version %s...\n", asset.filename, asset.version)
	if err := m.downloadFile(ctx, asset, tempFile); err != nil {
		return nil, err
	}

	if err := m.verifyChecksum(tempFile.Name(), asset.checksum); err != nil {
		return nil, err
	}

	if err := m.installBinary(tempFile.Name(), platform); err != nil {
		return nil, err
	}

	binaryPath := platform.binaryPath()
	color := m.io.Color()
	m.io.LogInfof("%s Installed to: %s\n", color.GreenCheck(), binaryPath)

	return &BinaryInfo{
		Path:     binaryPath,
		Version:  asset.version,
		Checksum: asset.checksum,
	}, nil
}

// fetchLatestPackage fetches the single latest Duo CLI package from the registry.
func (m *BinaryManager) fetchLatestPackage(ctx context.Context) (*gitlab.Package, error) {
	opts := &gitlab.ListProjectPackagesOptions{
		PackageType: new("generic"),
		PackageName: new(duoPackageName),
		OrderBy:     new("version"),
		Sort:        new("desc"),
		ListOptions: gitlab.ListOptions{PerPage: 1},
	}
	packages, _, err := m.client.Packages.ListProjectPackages(duoProjectID, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch packages: %w", err)
	}
	if len(packages) == 0 {
		return nil, errors.New("no packages found")
	}
	return packages[0], nil
}

// fetchPackageAsset fetches the package asset information from GitLab API using gitlab.Client.
func (m *BinaryManager) fetchPackageAsset(ctx context.Context, platform platform) (*packageAsset, error) {
	pkg, err := m.fetchLatestPackage(ctx)
	if err != nil {
		return nil, err
	}

	latestV, err := version.NewVersion(pkg.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid package version %q: %w", pkg.Version, err)
	}
	segs := latestV.Segments()
	if len(segs) == 0 || segs[0] > duoMaxCompatibleMajorVersion {
		return nil, fmt.Errorf("no compatible Duo CLI version found (major version %d supported; %s requires a newer glab)", duoMaxCompatibleMajorVersion, pkg.Version)
	}

	files, _, err := m.client.Packages.ListPackageFiles(duoProjectID, pkg.ID, &gitlab.ListPackageFilesOptions{}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package files: %w", err)
	}

	binaryName := platform.binaryName()
	idx := slices.IndexFunc(files, func(file *gitlab.PackageFile) bool {
		return file.FileName == binaryName
	})

	if idx == -1 {
		return nil, fmt.Errorf("binary not found for platform: %s", binaryName)
	}

	file := files[idx]
	return &packageAsset{
		version:  pkg.Version,
		filename: file.FileName,
		checksum: file.FileSHA256,
	}, nil
}

// downloadFile downloads the binary from GitLab using gitlab.Client.
func (m *BinaryManager) downloadFile(ctx context.Context, asset *packageAsset, dest *os.File) error {
	data, _, err := m.client.GenericPackages.DownloadPackageFile(
		duoProjectID,
		duoPackageName,
		asset.version,
		asset.filename,
		gitlab.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	if _, err := dest.Write(data); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// verifyChecksum verifies the SHA256 checksum of the downloaded file.
func (m *BinaryManager) verifyChecksum(filePath, expectedChecksum string) error {
	if expectedChecksum == "" {
		color := m.io.Color()
		m.io.LogInfof("%s No checksum available, skipping verification\n", color.DotWarnIcon())
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer file.Close() // Read-only file, error can be safely ignored

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum verification failed: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	color := m.io.Color()
	m.io.LogInfof("%s Checksum verified\n", color.GreenCheck())
	return nil
}

// installBinary installs the binary to the platform-specific location using atomic operations.
func (m *BinaryManager) installBinary(tempPath string, platform platform) error {
	installDir := platform.installDir()

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	binaryPath := platform.binaryPath()

	tmpDest, err := os.CreateTemp(installDir, ".duo-install-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file in install directory: %w", err)
	}
	tmpDestName := tmpDest.Name()
	defer os.Remove(tmpDestName)

	tempFile, err := os.Open(tempPath)
	if err != nil {
		tmpDest.Close()
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer tempFile.Close()

	if _, err := io.Copy(tmpDest, tempFile); err != nil {
		tmpDest.Close()
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	if err := tmpDest.Chmod(0o755); err != nil {
		tmpDest.Close()
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Required on Windows
	if err := tmpDest.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpDestName, binaryPath); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	return nil
}
