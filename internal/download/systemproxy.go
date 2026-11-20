package download

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpproxy"
)

type systemProxyConfig struct {
	http, https, socks *url.URL
	exceptions         []string
	excludeSimple      bool
}

// ProxyForRequest uses non-empty HTTP(S) environment proxies, otherwise the system proxy.
func ProxyForRequest(req *http.Request) (*url.URL, error) {
	return resolveProxy(req, os.LookupEnv, http.ProxyFromEnvironment, cachedSystemProxy)
}

func resolveProxy(req *http.Request, lookupEnv func(string) (string, bool), environment func(*http.Request) (*url.URL, error), system func() systemProxyConfig) (*url.URL, error) {
	if localProxyHost(req.URL.Hostname()) {
		return nil, nil
	}
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		if value, _ := lookupEnv(key); value != "" {
			return environment(req)
		}
	}
	proxy := system().forURL(req.URL)
	if proxy == nil {
		return nil, nil
	}
	noProxy, _ := lookupEnv("NO_PROXY")
	if noProxy == "" {
		noProxy, _ = lookupEnv("no_proxy")
	}
	config := httpproxy.Config{HTTPProxy: proxy.String(), HTTPSProxy: proxy.String(), NoProxy: noProxy}
	return config.ProxyFunc()(req.URL)
}

func localProxyHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || net.ParseIP(host).IsLoopback()
}

func (p systemProxyConfig) forURL(u *url.URL) *url.URL {
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	if proxyBypassed(u.Hostname(), port, p.exceptions, p.excludeSimple) {
		return nil
	}
	switch u.Scheme {
	case "http":
		if p.http != nil {
			return p.http
		}
	case "https":
		if p.https != nil {
			return p.https
		}
	default:
		return nil
	}
	return p.socks
}

func proxyBypassed(host, port string, exceptions []string, excludeSimple bool) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if localProxyHost(host) || (excludeSimple && !strings.ContainsAny(host, ".:") && net.ParseIP(host) == nil) {
		return true
	}
	for _, exception := range exceptions {
		rule := strings.ToLower(strings.TrimSpace(exception))
		if ruleHost, rulePort, err := net.SplitHostPort(rule); err == nil {
			if rulePort != port {
				continue
			}
			rule = ruleHost
		}
		if rule == "<local>" && !strings.ContainsAny(host, ".:") && net.ParseIP(host) == nil {
			return true
		}
		if strings.Contains(rule, "/") {
			address, bits, ok := strings.Cut(rule, "/")
			if ok && !strings.Contains(address, ":") {
				for strings.Count(address, ".") < 3 {
					address += ".0"
				}
				rule = address + "/" + bits
			}
			if _, network, err := net.ParseCIDR(rule); err == nil && network.Contains(net.ParseIP(host)) {
				return true
			}
			continue
		}
		if host == strings.TrimSuffix(rule, ".") {
			return true
		}
		if matched, _ := path.Match(rule, host); matched {
			return true
		}
	}
	return false
}

func proxyEndpoint(raw, scheme string) *url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "://") {
		raw = scheme + "://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" {
		return nil
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil
		}
	}
	return u
}

func parseSCUtilProxy(output string) systemProxyConfig {
	values := map[string]string{}
	var exceptions []string
	depth, exceptionsDepth := 0, -1
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "}" {
			if depth == exceptionsDepth {
				exceptionsDepth = -1
			}
			depth--
			continue
		}
		key, value, ok := strings.Cut(line, " : ")
		if strings.HasSuffix(line, "{") {
			if depth == 1 && key == "ExceptionsList" && strings.HasPrefix(value, "<array>") {
				exceptionsDepth = depth + 1
			}
			depth++
			continue
		}
		if !ok {
			continue
		}
		if depth == exceptionsDepth {
			exceptions = append(exceptions, strings.TrimSpace(value))
		} else if depth == 1 {
			values[key] = strings.TrimSpace(value)
		}
	}
	endpoint := func(prefix, scheme string) *url.URL {
		if values[prefix+"Enable"] != "1" {
			return nil
		}
		port, err := strconv.Atoi(values[prefix+"Port"])
		if err != nil || port < 1 || port > 65535 || values[prefix+"Proxy"] == "" {
			return nil
		}
		return proxyEndpoint(net.JoinHostPort(values[prefix+"Proxy"], strconv.Itoa(port)), scheme)
	}
	return systemProxyConfig{http: endpoint("HTTP", "http"), https: endpoint("HTTPS", "http"), socks: endpoint("SOCKS", "socks5"), exceptions: exceptions, excludeSimple: values["ExcludeSimpleHostnames"] == "1"}
}

func parseWindowsProxy(enabled bool, server, override string) systemProxyConfig {
	p := systemProxyConfig{exceptions: strings.Split(override, ";")}
	if !enabled {
		return systemProxyConfig{}
	}
	if !strings.Contains(server, "=") {
		p.http = proxyEndpoint(server, "http")
		p.https = p.http
		return p
	}
	for _, entry := range strings.Split(server, ";") {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "http":
			p.http = proxyEndpoint(value, "http")
		case "https":
			p.https = proxyEndpoint(value, "http")
		case "socks":
			p.socks = proxyEndpoint(value, "socks5")
		}
	}
	return p
}

func newSystemProxyCache(load func() systemProxyConfig, now func() time.Time) func() systemProxyConfig {
	var mu sync.Mutex
	var expires time.Time
	var current systemProxyConfig
	return func() systemProxyConfig {
		mu.Lock()
		defer mu.Unlock()
		if !now().Before(expires) {
			current = load()
			expires = now().Add(7 * time.Second)
		}
		return current
	}
}

var cachedSystemProxy = newSystemProxyCache(readSystemProxy, time.Now)
