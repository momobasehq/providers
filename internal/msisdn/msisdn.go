package msisdn

import (
	"fmt"
	"strings"
)

var callingCodes = map[string]string{
	"UG": "256", "KE": "254", "RW": "250", "TZ": "255", "ZM": "260", "CM": "237", "CD": "243",
	"GH": "233", "MW": "265", "ET": "251", "CI": "225", "SN": "221", "NG": "234",
}

func Digits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

func E164(account, country string) (string, error) {
	d := Digits(account)
	if d == "" {
		return "", fmt.Errorf("mobile money account is required")
	}
	cc := callingCodes[strings.ToUpper(country)]
	if cc == "" {
		if strings.HasPrefix(strings.TrimSpace(account), "+") {
			return "+" + d, nil
		}
		return d, nil
	}
	d = strings.TrimPrefix(d, "00")
	if strings.HasPrefix(d, cc) {
		return "+" + d, nil
	}
	d = strings.TrimLeft(d, "0")
	if d == "" {
		return "", fmt.Errorf("invalid mobile money account")
	}
	return "+" + cc + d, nil
}

func Local(account, country string) (string, error) {
	e, err := E164(account, country)
	if err != nil {
		return "", err
	}
	d := strings.TrimPrefix(e, "+")
	if cc := callingCodes[strings.ToUpper(country)]; cc != "" {
		d = strings.TrimPrefix(d, cc)
	}
	if d == "" {
		return "", fmt.Errorf("invalid mobile money account")
	}
	return d, nil
}

func CountryCode(country string) string { return callingCodes[strings.ToUpper(country)] }
