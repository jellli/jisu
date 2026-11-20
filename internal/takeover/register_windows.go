package takeover

import "golang.org/x/sys/windows/registry"

func registerRegistry(root registry.Key, base, path string) error {
	for _, browser := range []string{`Google\Chrome`, `Microsoft\Edge`} {
		prefix := `Software\`
		if base != "" {
			prefix = base + `\` + prefix
		}
		key, _, err := registry.CreateKey(root, prefix+browser+`\NativeMessagingHosts\`+HostName, registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = key.SetStringValue("", path)
		key.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
func registerPlatform(path string) error { return registerRegistry(registry.CURRENT_USER, "", path) }
