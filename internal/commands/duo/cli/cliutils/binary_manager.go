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
	"strings"
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

// CheckForUpdate checks if a newer version of Duo CLI is available.
// Returns (hasUpdate, latestVersion, newCheckTime, error).
// Caller should save newCheckTime to config if non-zero.
func (m *BinaryManager) CheckForUpdate(ctx context.Context, currentVersion string, lastCheckTime time.Time, forceCheck bool) (bool, string, time.Time, error) {
	if !forceCheck && !lastCheckTime.IsZero() && time.Since(lastCheckTime) < defaultUpdateCheckDelay {
		return false, "", time.Time{}, nil
	}

	latestVersion, err := m.getLatestVersion(ctx)
	if err != nil {
		return false, "", time.Time{}, err
	}

	newCheckTime := time.Now()

	current, err := version.NewVersion(strings.TrimPrefix(currentVersion, "v"))
	if err != nil {
		return false, "", newCheckTime, fmt.Errorf("invalid current version: %w", err)
	}

	latest, err := version.NewVersion(strings.TrimPrefix(latestVersion, "v"))
	if err != nil {
		return false, "", newCheckTime, fmt.Errorf("invalid latest version: %w", err)
	}

	if latest.GreaterThan(current) {
		return true, latestVersion, newCheckTime, nil
	}

	return false, latestVersion, newCheckTime, nil
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

// latestPackageOptions returns options for fetching the latest Duo CLI package.
func latestPackageOptions() *gitlab.ListProjectPackagesOptions {
	return &gitlab.ListProjectPackagesOptions{
		PackageType: new("generic"),
		PackageName: new(duoPackageName),
		OrderBy:     new("version"),
		Sort:        new("desc"),
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
		},
	}
}

// fetchPackageAsset fetches the package asset information from GitLab API using gitlab.Client.
func (m *BinaryManager) fetchPackageAsset(ctx context.Context, platform platform) (*packageAsset, error) {
	packages, _, err := m.client.Packages.ListProjectPackages(duoProjectID, latestPackageOptions(), gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch packages: %w", err)
	}

	if len(packages) == 0 {
		return nil, errors.New("no Duo CLI packages found")
	}

	latestPackage := packages[0]

	files, _, err := m.client.Packages.ListPackageFiles(duoProjectID, latestPackage.ID, &gitlab.ListPackageFilesOptions{}, gitlab.WithContext(ctx))
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
		version:  latestPackage.Version,
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

// getLatestVersion fetches the latest version from GitLab API using gitlab.Client.
func (m *BinaryManager) getLatestVersion(ctx context.Context) (string, error) {
	packages, _, err := m.client.Packages.ListProjectPackages(duoProjectID, latestPackageOptions(), gitlab.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to fetch packages: %w", err)
	}

	if len(packages) == 0 {
		return "", errors.New("no packages found")
	}

	return packages[0].Version, nil
}
