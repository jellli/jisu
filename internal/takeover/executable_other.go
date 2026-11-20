//go:build !darwin

package takeover

func stableExecutable(path string) (string, error) { return path, nil }
