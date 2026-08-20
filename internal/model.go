package internal

const (
	directSource = iota
	githubLatestReleaseArchive
)

// SourceSpec 描述一个明确需要的上游文件、Release 资产或源码归档。
type SourceSpec struct {
	Path       string
	URL        string
	Repository string
	Asset      string
	Kind       int
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

// SourceState 记录本次同步后的源文件、索引记录和更新状态。
type SourceState struct {
	Spec    SourceSpec
	Files   map[string][]byte
	Record  SourceRecord
	Updated bool
}

type Names struct {
	English string `json:"en"`
	Chinese string `json:"zh"`
}

type Region struct {
	ISO          string   `json:"iso"`
	ISO3         string   `json:"iso3,omitempty"`
	Flag         string   `json:"flag"`
	CallingCodes []string `json:"callingCodes"`
	Names        Names    `json:"names"`
	Numeric      string   `json:"numeric"`
	M49          M49Index `json:"m49"`
}

type M49Index struct {
	Region    string `json:"region,omitempty"`
	Subregion string `json:"subregion,omitempty"`
}

type M49Node struct {
	Code     string     `json:"code"`
	Names    Names      `json:"name"`
	Children []*M49Node `json:"children,omitempty"`
	Regions  []string   `json:"regions,omitempty"`
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
		{Path: "cldr/supplementalData.xml", URL: "https://raw.githubusercontent.com/unicode-org/cldr/main/common/supplemental/supplementalData.xml", Provenance: "Unicode CLDR (Unicode License v3)"},
		{Path: "libphonenumber/PhoneNumberMetadata.xml", URL: phoneURL, Provenance: "Google libphonenumber (Apache-2.0)"},
		{Path: "flag-icons", Repository: "lipis/flag-icons", Kind: githubLatestReleaseArchive, Provenance: "flag-icons (MIT)"},
	}
}
