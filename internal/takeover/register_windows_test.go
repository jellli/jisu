package takeover

import (
	"fmt"
	"golang.org/x/sys/windows/registry"
	"testing"
	"time"
)

func TestWindowsRegistry(t *testing.T) {
	base := fmt.Sprintf(`Software\jisu-test-%d`, time.Now().UnixNano())
	defer func() {
		for _, browser := range []string{`Google\Chrome`, `Microsoft\Edge`} {
			prefix := base + `\Software\` + browser
			registry.DeleteKey(registry.CURRENT_USER, prefix+`\NativeMessagingHosts\`+HostName)
			registry.DeleteKey(registry.CURRENT_USER, prefix+`\NativeMessagingHosts`)
			registry.DeleteKey(registry.CURRENT_USER, prefix)
		}
		registry.DeleteKey(registry.CURRENT_USER, base+`\Software\Google`)
		registry.DeleteKey(registry.CURRENT_USER, base+`\Software\Microsoft`)
		registry.DeleteKey(registry.CURRENT_USER, base+`\Software`)
		registry.DeleteKey(registry.CURRENT_USER, base)
	}()
	for range 2 {
		if err := registerRegistry(registry.CURRENT_USER, base, `C:\jisu\host.json`); err != nil {
			t.Fatal(err)
		}
	}
	for _, browser := range []string{`Google\Chrome`, `Microsoft\Edge`} {
		key, err := registry.OpenKey(registry.CURRENT_USER, base+`\Software\`+browser+`\NativeMessagingHosts\`+HostName, registry.QUERY_VALUE)
		if err != nil {
			t.Fatal(err)
		}
		value, _, err := key.GetStringValue("")
		key.Close()
		if err != nil || value != `C:\jisu\host.json` {
			t.Fatal(value, err)
		}
	}
}
