package fatsecret

import "time"

// secondsPerDay is the divisor FatSecret uses for its integer date format.
const secondsPerDay = 86400

// ToDateInt converts a time to FatSecret's "days since 1970-01-01" format.
// Uses the CALENDAR fields of t as seen in t's current location: midnight
// 11 May Moscow and 14:00 11 May UTC both map to dateInt for 11 May. The
// previous implementation used t.UTC().Unix()/86400, which silently shifted
// late-evening local times to the previous day's dateInt.
func ToDateInt(t time.Time) int {
	y, m, d := t.Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / secondsPerDay)
}

// FromDateInt is the inverse — returns midnight UTC of the given day count.
func FromDateInt(d int) time.Time {
	return time.Unix(int64(d)*secondsPerDay, 0).UTC()
}
