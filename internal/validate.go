package internal

import (
	"fmt"
	"regexp"
)

var (
	isoPattern  = regexp.MustCompile(`^[A-Z]{2}$`)
	iso3Pattern = regexp.MustCompile(`^[A-Z]{3}$`)
	codePattern = regexp.MustCompile(`^[1-9][0-9]*$`)

	// CLDR 的 territoryInfo 含有保留、用户分配和私有使用代码，均不属于 ISO 3166-1。
	nonISOTerritories = map[string]bool{
		"AC": true, "CP": true, "CQ": true, "DG": true, "EA": true, "EU": true, "EZ": true,
		"IC": true, "QO": true, "TA": true, "UN": true, "XK": true, "ZZ": true,
	}
)

func validISO(value string) bool  { return isoPattern.MatchString(value) }
func validISO3(value string) bool { return iso3Pattern.MatchString(value) }

func validateCountries(countries []Country) error {
	if len(countries) == 0 {
		return fmt.Errorf("no countries generated")
	}
	isos, iso3s := make(map[string]struct{}, len(countries)), make(map[string]struct{}, len(countries))
	for _, country := range countries {
		if !validISO(country.ISO) {
			return fmt.Errorf("invalid ISO %q", country.ISO)
		}
		if _, exists := isos[country.ISO]; exists {
			return fmt.Errorf("duplicate ISO %q", country.ISO)
		}
		isos[country.ISO] = struct{}{}
		if country.ISO3 != "" {
			if !validISO3(country.ISO3) {
				return fmt.Errorf("invalid ISO3 %q", country.ISO3)
			}
			if _, exists := iso3s[country.ISO3]; exists {
				return fmt.Errorf("duplicate ISO3 %q", country.ISO3)
			}
			iso3s[country.ISO3] = struct{}{}
		}
		if country.Names.English == "" {
			return fmt.Errorf("missing English name for %s", country.ISO)
		}
		for _, code := range country.CallingCodes {
			if !codePattern.MatchString(code) {
				return fmt.Errorf("invalid calling code %q", code)
			}
		}
	}
	return nil
}
