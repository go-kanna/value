package rule

// UUID rejects s with the code "uuid" unless it is a UUID in its standard
// textual form, "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", of any version. Hex
// digits may be in either case, as RFC 9562 allows on input.
func UUID(s string) error {
	if !isUUID(s) {
		return Fail("uuid")
	}
	return nil
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := range len(s) {
		switch i {
		case 8, 13, 18, 23:
			if s[i] != '-' {
				return false
			}
		default:
			if !isHex(s[i]) {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}
