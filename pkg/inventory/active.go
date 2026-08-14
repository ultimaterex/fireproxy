package inventory

// ActiveDevices returns Firewalla-visible hosts.
// If no device has Active stamped (legacy agent), returns devs unchanged.
func ActiveDevices(devs []Device) []Device {
	stamped := false
	for i := range devs {
		if devs[i].Active != nil {
			stamped = true
			break
		}
	}
	if !stamped {
		return devs
	}
	out := make([]Device, 0, len(devs))
	for _, d := range devs {
		if d.Active != nil && *d.Active {
			out = append(out, d)
		}
	}
	return out
}
