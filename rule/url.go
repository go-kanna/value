package rule

import "net/url"

// URL rejects s with the code "url" unless it is an absolute URL with both a
// scheme and a host, such as "https://example.com/path". A relative reference is
// rejected, and so is a URL without a host, such as "mailto:a@example.com" or
// "javascript:alert(1)".
func URL(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return Fail("url")
	}
	return nil
}
