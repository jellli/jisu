package download

import (
	"context"
	"os/exec"
	"time"
)

func readSystemProxy() systemProxyConfig {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/sbin/scutil", "--proxy").Output()
	if err != nil {
		return systemProxyConfig{}
	}
	return parseSCUtilProxy(string(output))
}
