package takeover

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func stableExecutable(path string) (string, error) {
	return resolveExecutable(path, originalBundlePath)
}

func resolveExecutable(path string, original func(string) (string, error)) (string, error) {
	if !strings.Contains(filepath.Clean(path), "/AppTranslocation/") {
		return path, nil
	}
	macOS := filepath.Dir(path)
	contents := filepath.Dir(macOS)
	bundle := filepath.Dir(contents)
	if filepath.Base(macOS) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(bundle, ".app") {
		return "", errors.New("cannot resolve translocated native host executable")
	}
	root, err := original(bundle)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(root) || strings.Contains(filepath.Clean(root), "/AppTranslocation/") || !strings.HasSuffix(root, ".app") {
		return "", errors.New("invalid original app bundle path")
	}
	stable := filepath.Join(root, "Contents", "MacOS", filepath.Base(path))
	info, err := os.Stat(stable)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", errors.New("original native host is not executable")
	}
	return stable, nil
}
