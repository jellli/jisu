package takeover

import (
	"os"
	"path/filepath"
	"runtime"
)

const HostName = "com.jellli.jisu"

const ExtensionID = "bgelplhjgnkddbiajjihhbacmnemgnhk"

func ManifestPaths(platform, home, config string) []string {
	var roots []string
	switch platform {
	case "darwin":
		roots = []string{
			filepath.Join(home, "Library", "Application Support", "Google", "Chrome"),
			filepath.Join(home, "Library", "Application Support", "Microsoft Edge"),
		}
	case "windows":
		return []string{filepath.Join(config, "jisu", HostName+".json")}
	default:
		roots = []string{
			filepath.Join(home, ".config", "google-chrome"),
			filepath.Join(home, ".config", "chromium"),
			filepath.Join(home, ".config", "microsoft-edge"),
		}
	}
	var paths []string
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(root, "NativeMessagingHosts", HostName+".json"))
	}
	return paths
}

func Register(home, config string) error {
	return register(home, config, registerPlatform)
}

func register(home, config string, platformRegister func(string) error) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	for _, path := range ManifestPaths(runtime.GOOS, home, config) {
		manifest := map[string]any{
			"name":            HostName,
			"description":     "jisu download Takeover",
			"path":            executable,
			"type":            "stdio",
			"allowed_origins": []string{"chrome-extension://" + ExtensionID + "/"},
		}
		if err = atomicJSON(path, manifest); err != nil {
			return err
		}
		if err = platformRegister(path); err != nil {
			return err
		}
	}
	return nil
}
