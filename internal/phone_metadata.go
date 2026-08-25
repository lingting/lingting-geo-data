package internal

type phoneMetadata struct {
	Territories []phoneTerritory `xml:"territories>territory"`
}

type phoneTerritory struct {
	ID                 string          `xml:"id,attr"`
	CountryCode        string          `xml:"countryCode,attr"`
	LeadingDigits      string          `xml:"leadingDigits,attr"`
	MainCountryForCode bool            `xml:"mainCountryForCode,attr"`
	FixedLine          phoneNumberType `xml:"fixedLine"`
	Mobile             phoneNumberType `xml:"mobile"`
}

type phoneNumberType struct {
	NationalNumberPattern string `xml:"nationalNumberPattern"`
}
