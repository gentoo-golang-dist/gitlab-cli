package binarymgr

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TarGzExtractor returns an Extractor that opens a .tar.gz archive, locates
// the entry whose basename matches binaryName, and writes it to destDir as
// an executable file. Symlinks and hardlinks are skipped. Entries whose
// resolved path escapes destDir are rejected (zip-slip).
func TarGzExtractor(binaryName string) Extractor {
	return func(srcPath, destDir string) (string, error) {
		f, err := os.Open(srcPath)
		if err != nil {
			return "", fmt.Errorf("failed to open archive: %w", err)
		}
		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", fmt.Errorf("failed to read gzip header: %w", err)
		}
		defer gz.Close()

		// destDir must exist for filepath.Abs round-trips to be meaningful.
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create extract directory: %w", err)
		}
		absDest, err := filepath.Abs(destDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve extract directory: %w", err)
		}

		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				return "", fmt.Errorf("archive does not contain %q", binaryName)
			}
			if err != nil {
				return "", fmt.Errorf("failed to read archive: %w", err)
			}

			if hdr.Typeflag != tar.TypeReg {
				continue
			}

			cleaned := filepath.Clean(hdr.Name)
			if strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
				return "", fmt.Errorf("archive entry %q escapes extract directory", hdr.Name)
			}
			if filepath.Base(cleaned) != binaryName {
				continue
			}

			outPath := filepath.Join(absDest, filepath.Base(cleaned))
			absOut, err := filepath.Abs(outPath)
			if err != nil {
				return "", fmt.Errorf("failed to resolve output path: %w", err)
			}
			if !strings.HasPrefix(absOut, absDest+string(filepath.Separator)) && absOut != absDest {
				return "", fmt.Errorf("archive entry %q resolves outside extract directory", hdr.Name)
			}

			out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return "", fmt.Errorf("failed to create extracted file: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", fmt.Errorf("failed to write extracted file: %w", err)
			}
			if err := out.Close(); err != nil {
				return "", fmt.Errorf("failed to close extracted file: %w", err)
			}
			return outPath, nil
		}
	}
}
