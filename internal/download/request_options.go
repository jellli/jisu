package download

import (
	"errors"
	"net/http"
	"net/url"
	"unicode"
)

type RequestOptions struct {
	Cookie    string `json:"-"`
	Referer   string `json:"referer,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
}

func (o *RequestOptions) validate() error {
	for _, v := range []struct {
		s   string
		max int
	}{{o.Cookie, 16384}, {o.Referer, 2048}, {o.UserAgent, 2048}} {
		if len(v.s) > v.max {
			return errors.New("request header is too long")
		}
		for _, c := range v.s {
			if unicode.IsControl(c) {
				return errors.New("invalid request header")
			}
		}
	}
	u, err := url.Parse(o.Referer)
	if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		o.Referer = ""
	}
	return nil
}

func (o RequestOptions) apply(req *http.Request) {
	if o.Cookie != "" {
		req.Header.Set("Cookie", o.Cookie)
	}
	if o.UserAgent != "" {
		req.Header.Set("User-Agent", o.UserAgent)
	}
	u, _ := url.Parse(o.Referer)
	if o.Referer != "" && u != nil && !(u.Scheme == "https" && req.URL.Scheme == "http") {
		req.Header.Set("Referer", o.Referer)
	}
}
