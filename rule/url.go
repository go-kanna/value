package rule

import (
	"net/url"
	"strconv"
	"strings"
)

// URL rejects s with the code "url" unless it is an absolute URL with both a
// scheme and a host name, such as "https://example.com/path". A relative
// reference is rejected, and so is a URL without a host name, such as
// "mailto:a@example.com" or "https://:80", a port above 65535, or a space.
//
// URL does not check the scheme: "javascript://example.com/%0Aalert(1)" passes.
// Before writing a URL into a link, check its scheme yourself.
func URL(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Hostname() == "" || strings.Contains(s, " ") || !validPort(u.Port()) {
		return Fail("url")
	}
	return nil
}

func validPort(p string) bool {
	if p == "" {
		return true
	}
	_, err := strconv.ParseUint(p, 10, 16)
	return err == nil
}
