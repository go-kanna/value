package rule

import "net/mail"

// Email rejects s with the code "email" unless it is a bare email address such
// as "a@example.com", as net/mail parses one. A display name, angle brackets, a
// comment, or surrounding space is rejected, so that an accepted s can be used
// as an address as it stands.
func Email(s string) error {
	a, err := mail.ParseAddress(s)
	if err != nil || a.Name != "" || a.Address != s {
		return Fail("email")
	}
	return nil
}
