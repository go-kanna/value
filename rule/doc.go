// Package rule provides what the validate method of a //kanna:value type is
// written with: ready-made rules such as Email, and Fail and FailWith for rules
// of one's own. A rule rejects a value with an *Error, whose code names the
// rule in a form stable enough to key a translated message or an API response.
//
// A rule is a plain function, so rules combine as any Go code does:
//
//	func (CompanyEmail) validate(s string) error {
//		if err := rule.Email(s); err != nil {
//			return err
//		}
//		if !strings.HasSuffix(s, "@example.com") {
//			return rule.Fail("corp_domain")
//		}
//		return nil
//	}
//
// The package depends on nothing outside the standard library.
package rule
