package structs

type MMDB_ASN struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

type MMDB_City struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Continent struct {
		Code  string            `maxminddb:"code"`
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"continent"`
	Country struct {
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
		IsoCode           string            `maxminddb:"iso_code"`
		Names             map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	RepresentedCountry struct {
		Names             map[string]string `maxminddb:"names"`
		IsoCode           string            `maxminddb:"iso_code"`
		Type              string            `maxminddb:"type"`
		IsInEuropeanUnion bool              `maxminddb:"is_in_european_union"`
	} `maxminddb:"represented_country"`
	Subdivisions []struct {
		IsoCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	Postal struct {
		Code string `maxminddb:"code"`
	} `maxminddb:"postal"`
	Location struct {
		Latitude       float64 `maxminddb:"latitude"`
		Longitude      float64 `maxminddb:"longitude"`
		TimeZone       string  `maxminddb:"time_zone"`
		MetroCode      uint    `maxminddb:"metro_code"`
		AccuracyRadius uint16  `maxminddb:"accuracy_radius"`
	} `maxminddb:"location"`
	Traits struct {
		IsAnonymousProxy    bool `maxminddb:"is_anonymous_proxy"`
		IsSatelliteProvider bool `maxminddb:"is_satellite_provider"`
	} `maxminddb:"traits"`
}
