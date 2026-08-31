package internal

const (
	directSource = iota
	githubLatestReleaseArchive
	cldrBase = "https://raw.githubusercontent.com/unicode-org/cldr-json/main/cldr-json/"
	phoneURL = "https://raw.githubusercontent.com/google/libphonenumber/master/resources/PhoneNumberMetadata.xml"
)

type SourceSpec struct {
	Path, URL, Repository, Asset string
	Kind                         int
	Provenance                   string
}
type SourceRecord struct {
	Path       string `json:"path"`
	URL        string `json:"url"`
	ETag       string `json:"etag,omitempty"`
	SHA256     string `json:"sha256"`
	Provenance string `json:"provenance"`
	Updated    bool   `json:"updated"`
}

type Resource struct {
	Spec    SourceSpec
	Files   map[string][]byte
	Record  SourceRecord
	Updated bool
}
type Resources struct {
	CldrCodeMappings     Resource
	CldrEnTerritories    Resource
	CldrZhTerritories    Resource
	CldrSupplementalData Resource
	PhoneNumberMetadata  Resource
	FlagIcons            Resource
}
type SourceIndex struct {
	CldrCodeMappings     SourceRecord `json:"cldrCodeMappings"`
	CldrEnTerritories    SourceRecord `json:"cldrEnTerritories"`
	CldrZhTerritories    SourceRecord `json:"cldrZhTerritories"`
	CldrSupplementalData SourceRecord `json:"cldrSupplementalData"`
	PhoneNumberMetadata  SourceRecord `json:"phoneNumberMetadata"`
	FlagIcons            SourceRecord `json:"flagIcons"`
}
type M49Generation struct {
	Root    *M49Node
	Indexes map[string]M49Index
	Updated bool
}
type PhoneGeneration struct {
	Regions       map[string][]PhoneNumber
	CallingCodes  map[string][]string
	PhonePrefixes map[string][]string
	Updated       bool
}
type PhoneNumber struct {
	Calling  int
	Prefixes []int
}
type RegionGeneration struct {
	Regions []Region
	Updated bool
}
type Names struct {
	English string `json:"en"`
	Chinese string `json:"zh"`
}
type Region struct {
	ISO           string   `json:"iso"`
	ISO3          string   `json:"iso3,omitempty"`
	Flag          string   `json:"flag"`
	CallingCodes  []string `json:"callingCodes"`
	PhonePrefixes []string `json:"phonePrefixes"`
	Names         Names    `json:"names"`
	Numeric       string   `json:"numeric"`
	M49           M49Index `json:"m49"`
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
type M49ParseResult struct {
	Root    *M49Node
	Indexes map[string]M49Index
}

func baseSources() Resources {
	return Resources{
		CldrCodeMappings:     Resource{Spec: SourceSpec{Path: "cldr/codeMappings.json", URL: cldrBase + "cldr-core/supplemental/codeMappings.json", Provenance: "Unicode CLDR (Unicode License v3)"}},
		CldrEnTerritories:    Resource{Spec: SourceSpec{Path: "cldr/en-territories.json", URL: cldrBase + "cldr-localenames-full/main/en/territories.json", Provenance: "Unicode CLDR (Unicode License v3)"}},
		CldrZhTerritories:    Resource{Spec: SourceSpec{Path: "cldr/zh-territories.json", URL: cldrBase + "cldr-localenames-full/main/zh/territories.json", Provenance: "Unicode CLDR (Unicode License v3)"}},
		CldrSupplementalData: Resource{Spec: SourceSpec{Path: "cldr/supplementalData.xml", URL: "https://raw.githubusercontent.com/unicode-org/cldr/main/common/supplemental/supplementalData.xml", Provenance: "Unicode CLDR (Unicode License v3)"}},
		PhoneNumberMetadata:  Resource{Spec: SourceSpec{Path: "libphonenumber/PhoneNumberMetadata.xml", URL: phoneURL, Provenance: "Google libphonenumber (Apache-2.0)"}},
		FlagIcons:            Resource{Spec: SourceSpec{Path: "flag-icons", Repository: "lipis/flag-icons", Kind: githubLatestReleaseArchive, Provenance: "flag-icons (MIT)"}},
	}
}
