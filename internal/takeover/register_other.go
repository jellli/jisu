//go:build !windows

package takeover

func registerPlatform(string) error { return nil }
