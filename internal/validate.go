package internal

import (
	"fmt"
	"regexp"
)

var (
	isoPattern  = regexp.MustCompile(`^[A-Z]{2}$`)
	iso3Pattern = regexp.MustCompile(`^[A-Z]{3}$`)
	codePattern = regexp.MustCompile(`^[1-9][0-9]*$`)

	m49Pattern = regexp.MustCompile(`^[0-9]{3}$`)

	// CLDR 包含保留、用户分配、私有使用和伪区域代码，均不属于 ISO 3166-1 区域。
	nonISORegions = map[string]bool{
		"AC": true, "CP": true, "CQ": true, "DG": true, "EA": true, "EU": true, "EZ": true,
		"IC": true, "QO": true, "TA": true, "UN": true, "XA": true, "XB": true, "XK": true, "ZZ": true,
	}
)

func validISO(value string) bool  { return isoPattern.MatchString(value) }
func validISO3(value string) bool { return iso3Pattern.MatchString(value) }
func validM49(value string) bool  { return m49Pattern.MatchString(value) }

func isISORegion(iso string, mapping codeMapping) bool {
	return validISO(iso) &&

		validISO3(mapping.Alpha3) &&

		validM49(mapping.Numeric) &&

		!nonISORegions[iso]
}
func validateRegions(regions []Region) error {
	if len(regions) == 0 {
		return fmt.Errorf("no regions generated")
	}
	isos, iso3s := make(map[string]struct{}, len(regions)), make(map[string]struct{}, len(regions))
	for _, region := range regions {
		if !validISO(region.ISO) {
			return fmt.Errorf("invalid ISO %q", region.ISO)
		}
		if _, exists := isos[region.ISO]; exists {
			return fmt.Errorf("duplicate ISO %q", region.ISO)
		}
		isos[region.ISO] = struct{}{}
		if region.Flag == "" {
			return fmt.Errorf("missing flag for %s", region.ISO)
		}
		if region.ISO3 != "" {
			if !validISO3(region.ISO3) {
				return fmt.Errorf("invalid ISO3 %q", region.ISO3)
			}
			if _, exists := iso3s[region.ISO3]; exists {
				return fmt.Errorf("duplicate ISO3 %q", region.ISO3)
			}
			iso3s[region.ISO3] = struct{}{}
		}
		if region.Names.English == "" {
			return fmt.Errorf("missing English name for %s", region.ISO)
		}
		if !validM49(region.Numeric) {
			return fmt.Errorf("invalid numeric M.49 %q", region.Numeric)
		}
		if region.M49.Region != "" && !validM49(region.M49.Region) {
			return fmt.Errorf("invalid M.49 region for %s", region.ISO)
		}
		if region.M49.Subregion != "" && !validM49(region.M49.Subregion) {
			return fmt.Errorf("invalid M.49 subregion for %s", region.ISO)
		}
		for _, code := range region.CallingCodes {
			if !codePattern.MatchString(code) {
				return fmt.Errorf("invalid calling code %q", code)
			}
		}
		for _, prefix := range region.PhonePrefixes {
			if !codePattern.MatchString(prefix) {
				return fmt.Errorf("invalid phone prefix %q", prefix)
			}
		}
	}
	return nil
}
