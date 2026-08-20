package internal

const (
	directSource = iota
	githubLatestSourceArchive
)

// SourceSpec 描述一个明确需要的上游文件或源码归档。
type SourceSpec struct {
	Path       string
	URL        string
	Repository string
	Provenance string
}

type SourceRecord struct {
	URL        string `json:"url"`
	ETag       string `json:"etag,omitempty"`
	SHA256     string `json:"sha256"`
	Provenance string `json:"provenance,omitempty"`
}

type SourceIndex struct {
	Sources map[string]SourceRecord `json:"sources"`
}

type Names struct {
	English string `json:"en"`
	Chinese string `json:"zh"`
}

type Country struct {
	ISO          string   `json:"iso"`
	ISO3         string   `json:"iso3,omitempty"`
	CallingCodes []string `json:"callingCodes"`
	Names        Names    `json:"names"`
}

const (
	cldrBase = "https://raw.githubusercontent.com/unicode-org/cldr-json/main/cldr-json/"
	phoneURL = "https://raw.githubusercontent.com/google/libphonenumber/master/resources/PhoneNumberMetadata.xml"
)

func baseSources() []SourceSpec {
	return []SourceSpec{
		{Path: "cldr/codeMappings.json", URL: cldrBase + "cldr-core/supplemental/codeMappings.json", Provenance: "Unicode CLDR (Unicode License v3)"},
		{Path: "cldr/en-territories.json", URL: cldrBase + "cldr-localenames-full/main/en/territories.json", Provenance: "Unicode CLDR (Unicode License v3)"},
		{Path: "cldr/zh-territories.json", URL: cldrBase + "cldr-localenames-full/main/zh/territories.json", Provenance: "Unicode CLDR (Unicode License v3)"},
		{Path: "libphonenumber/PhoneNumberMetadata.xml", URL: phoneURL, Provenance: "Google libphonenumber (Apache-2.0)"},
	}
}
