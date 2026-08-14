package collect

import "fireproxy/pkg/geo"

func countryName(cc string) string { return geo.Name(cc) }
