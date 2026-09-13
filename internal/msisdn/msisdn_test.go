package msisdn

import "testing"

func TestDigits(t *testing.T) {
	for in, want := range map[string]string{
		"+256 700-123456": "256700123456",
		"(0700) 123.456":  "0700123456",
		"abc":             "",
		"":                "",
		// unicode.IsDigit accepts non-ASCII digits, so they survive verbatim.
		"٣٤٥": "٣٤٥",
	} {
		if got := Digits(in); got != want {
			t.Errorf("Digits(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestE164(t *testing.T) {
	for _, c := range []struct{ account, country, want string }{
		{"0700123456", "UG", "+256700123456"},     // trunk prefix
		{"+256700123456", "UG", "+256700123456"},  // already E.164
		{"256700123456", "UG", "+256700123456"},   // calling code, no plus
		{"00256700123456", "UG", "+256700123456"}, // international access prefix
		{"0700 123-456", "UG", "+256700123456"},   // punctuation
		{"0700123456", "ug", "+256700123456"},     // country is case-insensitive
		{"0712345678", "KE", "+254712345678"},
		{"08031234567", "NG", "+2348031234567"},
		{"0788123456", "RW", "+250788123456"},
		// Country is unknown, but the account carries its own calling code.
		{"+22790123456", "NE", "+22790123456"},
	} {
		got, err := E164(c.account, c.country)
		if err != nil || got != c.want {
			t.Errorf("E164(%q, %q) = %q, %v; want %q", c.account, c.country, got, err, c.want)
		}
	}
}

func TestE164Errors(t *testing.T) {
	for _, c := range []struct{ account, country, want string }{
		{"", "UG", "mobile money account is required"},
		{"   ", "UG", "mobile money account is required"},
		{"abc", "UG", "mobile money account is required"},
		{"0", "UG", "invalid mobile money account"},
		{"000", "UG", "invalid mobile money account"},
	} {
		got, err := E164(c.account, c.country)
		if err == nil {
			t.Errorf("E164(%q, %q) = %q, want error", c.account, c.country, got)
			continue
		}
		if err.Error() != c.want {
			t.Errorf("E164(%q, %q) error = %q, want %q", c.account, c.country, err, c.want)
		}
	}
}

func TestLocal(t *testing.T) {
	for _, c := range []struct{ account, country, want string }{
		{"0700123456", "UG", "700123456"},
		{"+256700123456", "UG", "700123456"},
		{"00256700123456", "UG", "700123456"},
		{"0712345678", "KE", "712345678"},
		{"08031234567", "NG", "8031234567"},
	} {
		got, err := Local(c.account, c.country)
		if err != nil || got != c.want {
			t.Errorf("Local(%q, %q) = %q, %v; want %q", c.account, c.country, got, err, c.want)
		}
	}
	if _, err := Local("", "UG"); err == nil {
		t.Error("Local should propagate the E164 error")
	}
}

func TestCountryCode(t *testing.T) {
	for country, want := range map[string]string{
		"UG": "256", "ug": "256", "KE": "254", "NG": "234",
		"ZZ": "", "": "",
		" GH ": "", // not trimmed, only upper-cased
	} {
		if got := CountryCode(country); got != want {
			t.Errorf("CountryCode(%q) = %q, want %q", country, got, want)
		}
	}
}

// TestKnownLimitations pins the gaps in this package rather than leaving them
// undocumented. Normalization here is a convenience: the provider owns the
// numbering plan for its market and performs the authoritative validation, so
// these inputs are forwarded and rejected upstream instead of locally.
//
// If these assertions ever start failing, the behaviour was tightened on
// purpose and the expectations below should be updated to match.
func TestKnownLimitations(t *testing.T) {
	// A country outside callingCodes yields a non-E.164 string, with no error.
	if got, err := E164("0790123456", "NE"); got != "0790123456" || err != nil {
		t.Errorf(`E164("0790123456", "NE") = %q, %v`, got, err)
	}
	// So does a missing country.
	if got, err := E164("0700123456", ""); got != "0700123456" || err != nil {
		t.Errorf(`E164("0700123456", "") = %q, %v`, got, err)
	}
	// Local cannot strip a calling code it does not know.
	if got, _ := Local("+22790123456", "NE"); got != "22790123456" {
		t.Errorf(`Local("+22790123456", "NE") = %q`, got)
	}
	// Trunk prefix plus calling code is not detected, so the code is applied twice.
	if got, _ := E164("0254712345678", "KE"); got != "+254254712345678" {
		t.Errorf(`E164("0254712345678", "KE") = %q`, got)
	}
	// A calling-code prefix short-circuits the trunk-prefix strip, keeping the 0.
	if got, _ := E164("2560700123456", "UG"); got != "+2560700123456" {
		t.Errorf(`E164("2560700123456", "UG") = %q`, got)
	}
}
