// Package geo maps ISO country codes to display names.
// MaxMind MMDB lookup lives on the server, not here.
package geo

import (
	"encoding/json"
	"strings"
	"unicode"
)

type rec struct {
	A2, A3, Name string
}

// Name returns a display name for an ISO 3166-1 alpha-2 code.
func Name(cc string) string {
	cc = Code(cc)
	if cc == "" {
		return ""
	}
	if n, ok := byA2[cc]; ok {
		return n.Name
	}
	return cc
}

// Code normalizes intel / MMDB / name input to ISO 3166-1 alpha-2.
// Junk like "[]" or "XX" is empty so GeoLite can fill the gap.
func Code(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" || s == "null" || s == "-" || s == "N/A" {
		return ""
	}
	if strings.HasPrefix(s, "[") {
		var arr []string
		if json.Unmarshal([]byte(s), &arr) == nil {
			for _, p := range arr {
				if c := Code(p); c != "" {
					return c
				}
			}
		}
		return ""
	}
	up := strings.ToUpper(s)
	if len(up) == 2 && isAlpha(up) {
		if up == "XX" || up == "ZZ" || up == "AA" {
			return ""
		}
		if _, ok := byA2[up]; ok {
			return up
		}
		return ""
	}
	if len(up) == 3 && isAlpha(up) {
		if a2, ok := byA3[up]; ok {
			return a2
		}
		return ""
	}
	if a2, ok := byName[strings.ToLower(s)]; ok {
		return a2
	}
	return ""
}

func isAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

var (
	byA2   = map[string]rec{}
	byA3   = map[string]string{}
	byName = map[string]string{}
)

func init() {
	for _, r := range iso {
		byA2[r.A2] = r
		if r.A3 != "" {
			byA3[r.A3] = r.A2
		}
		byName[strings.ToLower(r.Name)] = r.A2
	}
	for name, a2 := range aliases {
		byName[name] = a2
	}
}

var aliases = map[string]string{
	"usa": "US", "united states of america": "US", "america": "US",
	"uk": "GB", "great britain": "GB", "britain": "GB", "england": "GB",
	"holland": "NL", "the netherlands": "NL",
	"russia": "RU", "south korea": "KR", "north korea": "KP",
	"vietnam": "VN", "ivoire": "CI", "ivory coast": "CI",
	"czech republic": "CZ", "swaziland": "SZ",
	"macedonia": "MK", "palestine": "PS",
	"taiwan, province of china": "TW", "korea, republic of": "KR",
	"russian federation": "RU", "syrian arab republic": "SY",
}

var iso = []rec{
	{"AD", "AND", "Andorra"}, {"AE", "ARE", "United Arab Emirates"}, {"AF", "AFG", "Afghanistan"},
	{"AG", "ATG", "Antigua and Barbuda"}, {"AI", "AIA", "Anguilla"}, {"AL", "ALB", "Albania"},
	{"AM", "ARM", "Armenia"}, {"AO", "AGO", "Angola"}, {"AQ", "ATA", "Antarctica"},
	{"AR", "ARG", "Argentina"}, {"AS", "ASM", "American Samoa"}, {"AT", "AUT", "Austria"},
	{"AU", "AUS", "Australia"}, {"AW", "ABW", "Aruba"}, {"AX", "ALA", "Åland Islands"},
	{"AZ", "AZE", "Azerbaijan"}, {"BA", "BIH", "Bosnia and Herzegovina"}, {"BB", "BRB", "Barbados"},
	{"BD", "BGD", "Bangladesh"}, {"BE", "BEL", "Belgium"}, {"BF", "BFA", "Burkina Faso"},
	{"BG", "BGR", "Bulgaria"}, {"BH", "BHR", "Bahrain"}, {"BI", "BDI", "Burundi"},
	{"BJ", "BEN", "Benin"}, {"BL", "BLM", "Saint Barthélemy"}, {"BM", "BMU", "Bermuda"},
	{"BN", "BRN", "Brunei"}, {"BO", "BOL", "Bolivia"}, {"BQ", "BES", "Caribbean Netherlands"},
	{"BR", "BRA", "Brazil"}, {"BS", "BHS", "Bahamas"}, {"BT", "BTN", "Bhutan"},
	{"BV", "BVT", "Bouvet Island"}, {"BW", "BWA", "Botswana"}, {"BY", "BLR", "Belarus"},
	{"BZ", "BLZ", "Belize"}, {"CA", "CAN", "Canada"}, {"CC", "CCK", "Cocos Islands"},
	{"CD", "COD", "DR Congo"}, {"CF", "CAF", "Central African Republic"}, {"CG", "COG", "Congo"},
	{"CH", "CHE", "Switzerland"}, {"CI", "CIV", "Côte d'Ivoire"}, {"CK", "COK", "Cook Islands"},
	{"CL", "CHL", "Chile"}, {"CM", "CMR", "Cameroon"}, {"CN", "CHN", "China"},
	{"CO", "COL", "Colombia"}, {"CR", "CRI", "Costa Rica"}, {"CU", "CUB", "Cuba"},
	{"CV", "CPV", "Cabo Verde"}, {"CW", "CUW", "Curaçao"}, {"CX", "CXR", "Christmas Island"},
	{"CY", "CYP", "Cyprus"}, {"CZ", "CZE", "Czechia"}, {"DE", "DEU", "Germany"},
	{"DJ", "DJI", "Djibouti"}, {"DK", "DNK", "Denmark"}, {"DM", "DMA", "Dominica"},
	{"DO", "DOM", "Dominican Republic"}, {"DZ", "DZA", "Algeria"}, {"EC", "ECU", "Ecuador"},
	{"EE", "EST", "Estonia"}, {"EG", "EGY", "Egypt"}, {"EH", "ESH", "Western Sahara"},
	{"ER", "ERI", "Eritrea"}, {"ES", "ESP", "Spain"}, {"ET", "ETH", "Ethiopia"},
	{"EU", "", "European Union"}, {"FI", "FIN", "Finland"}, {"FJ", "FJI", "Fiji"},
	{"FK", "FLK", "Falkland Islands"}, {"FM", "FSM", "Micronesia"}, {"FO", "FRO", "Faroe Islands"},
	{"FR", "FRA", "France"}, {"GA", "GAB", "Gabon"}, {"GB", "GBR", "United Kingdom"},
	{"GD", "GRD", "Grenada"}, {"GE", "GEO", "Georgia"}, {"GF", "GUF", "French Guiana"},
	{"GG", "GGY", "Guernsey"}, {"GH", "GHA", "Ghana"}, {"GI", "GIB", "Gibraltar"},
	{"GL", "GRL", "Greenland"}, {"GM", "GMB", "Gambia"}, {"GN", "GIN", "Guinea"},
	{"GP", "GLP", "Guadeloupe"}, {"GQ", "GNQ", "Equatorial Guinea"}, {"GR", "GRC", "Greece"},
	{"GS", "SGS", "South Georgia"}, {"GT", "GTM", "Guatemala"}, {"GU", "GUM", "Guam"},
	{"GW", "GNB", "Guinea-Bissau"}, {"GY", "GUY", "Guyana"}, {"HK", "HKG", "Hong Kong"},
	{"HM", "HMD", "Heard Island"}, {"HN", "HND", "Honduras"}, {"HR", "HRV", "Croatia"},
	{"HT", "HTI", "Haiti"}, {"HU", "HUN", "Hungary"}, {"ID", "IDN", "Indonesia"},
	{"IE", "IRL", "Ireland"}, {"IL", "ISR", "Israel"}, {"IM", "IMN", "Isle of Man"},
	{"IN", "IND", "India"}, {"IO", "IOT", "British Indian Ocean Territory"}, {"IQ", "IRQ", "Iraq"},
	{"IR", "IRN", "Iran"}, {"IS", "ISL", "Iceland"}, {"IT", "ITA", "Italy"},
	{"JE", "JEY", "Jersey"}, {"JM", "JAM", "Jamaica"}, {"JO", "JOR", "Jordan"},
	{"JP", "JPN", "Japan"}, {"KE", "KEN", "Kenya"}, {"KG", "KGZ", "Kyrgyzstan"},
	{"KH", "KHM", "Cambodia"}, {"KI", "KIR", "Kiribati"}, {"KM", "COM", "Comoros"},
	{"KN", "KNA", "Saint Kitts and Nevis"}, {"KP", "PRK", "North Korea"}, {"KR", "KOR", "South Korea"},
	{"KW", "KWT", "Kuwait"}, {"KY", "CYM", "Cayman Islands"}, {"KZ", "KAZ", "Kazakhstan"},
	{"LA", "LAO", "Laos"}, {"LB", "LBN", "Lebanon"}, {"LC", "LCA", "Saint Lucia"},
	{"LI", "LIE", "Liechtenstein"}, {"LK", "LKA", "Sri Lanka"}, {"LR", "LBR", "Liberia"},
	{"LS", "LSO", "Lesotho"}, {"LT", "LTU", "Lithuania"}, {"LU", "LUX", "Luxembourg"},
	{"LV", "LVA", "Latvia"}, {"LY", "LBY", "Libya"}, {"MA", "MAR", "Morocco"},
	{"MC", "MCO", "Monaco"}, {"MD", "MDA", "Moldova"}, {"ME", "MNE", "Montenegro"},
	{"MF", "MAF", "Saint Martin"}, {"MG", "MDG", "Madagascar"}, {"MH", "MHL", "Marshall Islands"},
	{"MK", "MKD", "North Macedonia"}, {"ML", "MLI", "Mali"}, {"MM", "MMR", "Myanmar"},
	{"MN", "MNG", "Mongolia"}, {"MO", "MAC", "Macao"}, {"MP", "MNP", "Northern Mariana Islands"},
	{"MQ", "MTQ", "Martinique"}, {"MR", "MRT", "Mauritania"}, {"MS", "MSR", "Montserrat"},
	{"MT", "MLT", "Malta"}, {"MU", "MUS", "Mauritius"}, {"MV", "MDV", "Maldives"},
	{"MW", "MWI", "Malawi"}, {"MX", "MEX", "Mexico"}, {"MY", "MYS", "Malaysia"},
	{"MZ", "MOZ", "Mozambique"}, {"NA", "NAM", "Namibia"}, {"NC", "NCL", "New Caledonia"},
	{"NE", "NER", "Niger"}, {"NF", "NFK", "Norfolk Island"}, {"NG", "NGA", "Nigeria"},
	{"NI", "NIC", "Nicaragua"}, {"NL", "NLD", "Netherlands"}, {"NO", "NOR", "Norway"},
	{"NP", "NPL", "Nepal"}, {"NR", "NRU", "Nauru"}, {"NU", "NIU", "Niue"},
	{"NZ", "NZL", "New Zealand"}, {"OM", "OMN", "Oman"}, {"PA", "PAN", "Panama"},
	{"PE", "PER", "Peru"}, {"PF", "PYF", "French Polynesia"}, {"PG", "PNG", "Papua New Guinea"},
	{"PH", "PHL", "Philippines"}, {"PK", "PAK", "Pakistan"}, {"PL", "POL", "Poland"},
	{"PM", "SPM", "Saint Pierre and Miquelon"}, {"PN", "PCN", "Pitcairn"}, {"PR", "PRI", "Puerto Rico"},
	{"PS", "PSE", "Palestine"}, {"PT", "PRT", "Portugal"}, {"PW", "PLW", "Palau"},
	{"PY", "PRY", "Paraguay"}, {"QA", "QAT", "Qatar"}, {"RE", "REU", "Réunion"},
	{"RO", "ROU", "Romania"}, {"RS", "SRB", "Serbia"}, {"RU", "RUS", "Russia"},
	{"RW", "RWA", "Rwanda"}, {"SA", "SAU", "Saudi Arabia"}, {"SB", "SLB", "Solomon Islands"},
	{"SC", "SYC", "Seychelles"}, {"SD", "SDN", "Sudan"}, {"SE", "SWE", "Sweden"},
	{"SG", "SGP", "Singapore"}, {"SH", "SHN", "Saint Helena"}, {"SI", "SVN", "Slovenia"},
	{"SJ", "SJM", "Svalbard and Jan Mayen"}, {"SK", "SVK", "Slovakia"}, {"SL", "SLE", "Sierra Leone"},
	{"SM", "SMR", "San Marino"}, {"SN", "SEN", "Senegal"}, {"SO", "SOM", "Somalia"},
	{"SR", "SUR", "Suriname"}, {"SS", "SSD", "South Sudan"}, {"ST", "STP", "São Tomé and Príncipe"},
	{"SV", "SLV", "El Salvador"}, {"SX", "SXM", "Sint Maarten"}, {"SY", "SYR", "Syria"},
	{"SZ", "SWZ", "Eswatini"}, {"TC", "TCA", "Turks and Caicos"}, {"TD", "TCD", "Chad"},
	{"TF", "ATF", "French Southern Territories"}, {"TG", "TGO", "Togo"}, {"TH", "THA", "Thailand"},
	{"TJ", "TJK", "Tajikistan"}, {"TK", "TKL", "Tokelau"}, {"TL", "TLS", "Timor-Leste"},
	{"TM", "TKM", "Turkmenistan"}, {"TN", "TUN", "Tunisia"}, {"TO", "TON", "Tonga"},
	{"TR", "TUR", "Turkey"}, {"TT", "TTO", "Trinidad and Tobago"}, {"TV", "TUV", "Tuvalu"},
	{"TW", "TWN", "Taiwan"}, {"TZ", "TZA", "Tanzania"}, {"UA", "UKR", "Ukraine"},
	{"UG", "UGA", "Uganda"}, {"UM", "UMI", "U.S. Minor Outlying Islands"}, {"US", "USA", "United States"},
	{"UY", "URY", "Uruguay"}, {"UZ", "UZB", "Uzbekistan"}, {"VA", "VAT", "Vatican City"},
	{"VC", "VCT", "Saint Vincent"}, {"VE", "VEN", "Venezuela"}, {"VG", "VGB", "British Virgin Islands"},
	{"VI", "VIR", "U.S. Virgin Islands"}, {"VN", "VNM", "Vietnam"}, {"VU", "VUT", "Vanuatu"},
	{"WF", "WLF", "Wallis and Futuna"}, {"WS", "WSM", "Samoa"}, {"XK", "XKX", "Kosovo"},
	{"YE", "YEM", "Yemen"}, {"YT", "MYT", "Mayotte"}, {"ZA", "ZAF", "South Africa"},
	{"ZM", "ZMB", "Zambia"}, {"ZW", "ZWE", "Zimbabwe"},
}
