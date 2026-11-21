package sheet

import (
	"math"
	"time"
)

// Moon phases, from Jean Meeus, Astronomical Algorithms (2nd ed., 1998),
// chapter 49: the true new and full moons, accurate to a minute or so
// over the supported years, and the day-by-day phase drawn on a moon
// calendar, the fraction of the lunation (new moon to new moon) elapsed.

// moonEvent is a moon phase a calendar marks.
type moonEvent int

const (
	noMoonEvent moonEvent = iota
	newMoonEvent
	fullMoonEvent
)

func (e moonEvent) String() string {
	switch e {
	case newMoonEvent:
		return "new"
	case fullMoonEvent:
		return "full"
	}
	return "none"
}

// moonEventTime is a new or full moon and its time, in UTC.
type moonEventTime struct {
	Event moonEvent
	Time  time.Time
}

// moonDay is one calendar day of a moon calendar.
type moonDay struct {
	Day int // day of the month, from 1
	// Phase is the lunation fraction at local noon, in [0, 1): 0 new,
	// 0.25 first quarter, 0.5 full, 0.75 last quarter.
	Phase float64
	Event moonEvent // the new or full moon on this local date, if any
}

// Phase calculation constants.
const (
	// synodicMonth is the mean lunation, in days.
	synodicMonth = 29.530588861
	// unixEpochJD is the Julian day of 1970-01-01 00:00 UTC.
	unixEpochJD = 2440587.5
)

// moonPhaseJDE is the Julian Ephemeris Day (Terrestrial Time) of the true
// new moon of lunation k (an integer, 0 the new moon of 6 January 2000) or,
// for k + 0.5, the true full moon after it: Meeus (49.1) with the periodic
// terms and the planetary arguments of chapter 49.
func moonPhaseJDE(k float64) float64 {
	t := k / 1236.85
	t2, t3, t4 := t*t, t*t*t, t*t*t*t
	jde := 2451550.09766 + synodicMonth*k + 0.00015437*t2 - 0.000000150*t3 + 0.00000000073*t4

	rad := func(deg float64) float64 { return math.Mod(deg, 360) * math.Pi / 180 }
	e := 1 - 0.002516*t - 0.0000074*t2
	m := rad(2.5534 + 29.10535670*k - 0.0000014*t2 - 0.00000011*t3)
	mp := rad(201.5643 + 385.81693528*k + 0.0107582*t2 + 0.00001238*t3 - 0.000000058*t4)
	f := rad(160.7108 + 390.67050284*k - 0.0016118*t2 - 0.00000227*t3 + 0.000000011*t4)
	om := rad(124.7746 - 1.56375588*k + 0.0020672*t2 + 0.00000215*t3)
	sin := math.Sin

	// The first terms differ between new and full moon; the rest are shared.
	c := []float64{-0.40720, 0.17241, 0.01608, 0.01039, 0.00739, -0.00514, 0.00208}
	if math.Abs(k-math.Floor(k)-0.5) < 1e-9 {
		c = []float64{-0.40614, 0.17302, 0.01614, 0.01043, 0.00734, -0.00515, 0.00209}
	}
	jde += c[0]*sin(mp) +
		c[1]*e*sin(m) +
		c[2]*sin(2*mp) +
		c[3]*sin(2*f) +
		c[4]*e*sin(mp-m) +
		c[5]*e*sin(mp+m) +
		c[6]*e*e*sin(2*m) +
		-0.00111*sin(mp-2*f) +
		-0.00057*sin(mp+2*f) +
		0.00056*e*sin(2*mp+m) +
		-0.00042*sin(3*mp) +
		0.00042*e*sin(m+2*f) +
		0.00038*e*sin(m-2*f) +
		-0.00024*e*sin(2*mp-m) +
		-0.00017*sin(om) +
		-0.00007*sin(mp+2*m) +
		0.00004*sin(2*mp-2*f) +
		0.00004*sin(3*m) +
		0.00003*sin(mp+m-2*f) +
		0.00003*sin(2*mp+2*f) +
		-0.00003*sin(mp+m+2*f) +
		0.00003*sin(mp-m+2*f) +
		-0.00002*sin(mp-m-2*f) +
		-0.00002*sin(3*mp+m) +
		0.00002*sin(4*mp)

	// Additional corrections from the planetary arguments A1–A14.
	planetary := [][3]float64{
		{299.77, 0.107408, 0.000325},
		{251.88, 0.016321, 0.000165},
		{251.83, 26.651886, 0.000164},
		{349.42, 36.412478, 0.000126},
		{84.66, 18.206239, 0.000110},
		{141.74, 53.303771, 0.000062},
		{207.14, 2.453732, 0.000060},
		{154.84, 7.306860, 0.000056},
		{34.52, 27.261239, 0.000047},
		{207.19, 0.121824, 0.000042},
		{291.34, 1.844379, 0.000040},
		{161.72, 24.198154, 0.000037},
		{239.56, 25.513099, 0.000035},
		{331.55, 3.592518, 0.000023},
	}
	for i, a := range planetary {
		arg := a[0] + a[1]*k
		if i == 0 {
			arg -= 0.009173 * t2 // A1 has a T² term
		}
		jde += a[2] * sin(rad(arg))
	}
	return jde
}

// deltaT is ΔT = TT − UT in seconds for a decimal year: the polynomials of
// Espenak and Meeus (NASA's Five Millennium Canon of Solar Eclipses) for
// 1900–2150. Past 2005 they are extrapolations, off by a few seconds now
// and by a minute or two by 2100, which moves no event by a day.
func deltaT(y float64) float64 {
	switch {
	case y < 1920:
		t := y - 1900
		return -2.79 + 1.494119*t - 0.0598939*t*t + 0.0061966*t*t*t - 0.000197*t*t*t*t
	case y < 1941:
		t := y - 1920
		return 21.20 + 0.84493*t - 0.076100*t*t + 0.0020936*t*t*t
	case y < 1961:
		t := y - 1950
		return 29.07 + 0.407*t - t*t/233 + t*t*t/2547
	case y < 1986:
		t := y - 1975
		return 45.45 + 1.067*t - t*t/260 - t*t*t/718
	case y < 2005:
		t := y - 2000
		return 63.86 + 0.3345*t - 0.060374*t*t + 0.0017275*t*t*t + 0.000651814*t*t*t*t + 0.00002373599*t*t*t*t*t
	case y < 2050:
		t := y - 2000
		return 62.92 + 0.32217*t + 0.005589*t*t
	default:
		u := (y - 1820) / 100
		return -20 + 32*u*u - 0.5628*(2150-y)
	}
}

// moonPhaseTime is the time, in UTC, of the true new moon of lunation k
// or, for k + 0.5, of the full moon after it.
func moonPhaseTime(k float64) time.Time {
	jde := moonPhaseJDE(k)
	year := 2000 + (jde-2451545.0)/365.25
	days := jde - unixEpochJD - deltaT(year)/86400
	return time.Unix(0, int64(math.Round(days*86400))*int64(time.Second)).UTC()
}

// lunationAt is the lunation in progress at t: the integer k whose true
// new moon is at or before t, with the next one after t.
func lunationAt(t time.Time) float64 {
	jd := float64(t.Unix())/86400 + unixEpochJD
	k := math.Floor((jd - 2451550.09766) / synodicMonth)
	// The true new moon is within about 14 hours of the mean one.
	for moonPhaseTime(k).After(t) {
		k--
	}
	for !moonPhaseTime(k + 1).After(t) {
		k++
	}
	return k
}

// moonPhaseAt is the lunation fraction at t, in [0, 1): the time since the
// last true new moon over the time between it and the next.
func moonPhaseAt(t time.Time) float64 {
	k := lunationAt(t)
	start, end := moonPhaseTime(k), moonPhaseTime(k+1)
	return float64(t.Sub(start)) / float64(end.Sub(start))
}

// moonEvents lists the true new and full moons from from (inclusive) to to
// (exclusive), in order.
func moonEvents(from, to time.Time) []moonEventTime {
	var out []moonEventTime
	for k := lunationAt(from); ; k += 0.5 {
		when := moonPhaseTime(k)
		if !when.Before(to) {
			return out
		}
		if when.Before(from) {
			continue
		}
		event := newMoonEvent
		if k != math.Floor(k) {
			event = fullMoonEvent
		}
		out = append(out, moonEventTime{Event: event, Time: when})
	}
}

// daysIn is the number of days in month of year.
func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// moonMonth is the moon calendar of month of year in loc: every calendar
// day with its phase at local noon and the new or full moon on its local
// date.
func moonMonth(year int, month time.Month, loc *time.Location) []moonDay {
	n := daysIn(year, month)
	days := make([]moonDay, n)
	for i := range days {
		noon := time.Date(year, month, i+1, 12, 0, 0, 0, loc)
		days[i] = moonDay{Day: i + 1, Phase: moonPhaseAt(noon)}
	}
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	for _, e := range moonEvents(first, first.AddDate(0, 1, 0)) {
		days[e.Time.In(loc).Day()-1].Event = e.Event
	}
	return days
}
