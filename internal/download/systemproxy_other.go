//go:build !darwin && !windows

package download

func readSystemProxy() systemProxyConfig { return systemProxyConfig{} }
