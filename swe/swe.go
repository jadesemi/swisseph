// Package swe is the Go binding over the vendored Swiss Ephemeris C
// (Astrodienst 2.10.03), exposing exactly the calls the astro access layer
// needs: ephemeris-path setup, Julian-day conversion, a body's position, the
// house cusps with Ascendant and Midheaven — from a Julian day and longitude
// (Houses) or from a right ascension of the Midheaven and obliquity
// (HousesARMC) — and teardown. Every call returns
// Go types, the raw C return value, and the Swiss Ephemeris error string —
// never a C type.
//
// Swiss Ephemeris keeps all computation state in a C global declared
// thread-local, so SetEphePath, CalcUT, Houses, and Close operate on the state
// of the OS thread they run on: a path set on one thread is invisible to a
// calc on another. Every caller must therefore serialize these calls onto a
// single OS-locked thread; this binding does not do that itself. Julday alone
// is stateless calendar arithmetic and may be called from any thread.
//
// Swiss Ephemeris is used under the Swiss Ephemeris Professional License
// obtained from Astrodienst; the AGPL alternative is not exercised.
package swe

/*
#cgo CFLAGS: -I${SRCDIR}/.. -std=gnu11 -Wno-error=format-security -Wno-unused-result -Wno-format
#cgo LDFLAGS: -lm
#include <stdlib.h>
#include "swephexp.h"
*/
import "C"

import "unsafe"

const (
	// CGregCal selects the Gregorian calendar for Julday (Swiss Ephemeris
	// SE_GREG_CAL).
	CGregCal = 1

	// CFlagSwisseph requests Swiss Ephemeris computation mode, reading the .se1
	// files rather than the built-in Moshier fallback (Swiss Ephemeris
	// SEFLG_SWIEPH).
	CFlagSwisseph = 2

	// CFlagSpeed requests that CalcUT also compute each body's speeds alongside
	// its positions (Swiss Ephemeris SEFLG_SPEED).
	CFlagSpeed = 256

	// CHouseSystemPlacidus selects the Placidus house system for Houses, the
	// Swiss Ephemeris house-system letter 'P'.
	CHouseSystemPlacidus = 'P'
)

// cSerrSize is the Swiss Ephemeris error-string buffer length (AS_MAXCH): the
// maximum bytes swe_calc_ut writes into serr, including the terminating NUL.
const cSerrSize = 256

// SetEphePath points Swiss Ephemeris at the directory it reads .se1 ephemeris
// files from, on the calling OS thread. An empty path selects the built-in
// default search path. Because the path is stored in thread-local state, it
// must be called on the same OS thread as the CalcUT and Houses calls that
// follow it.
func SetEphePath(path string) {
	if path == "" {
		C.swe_set_ephe_path(nil)
		return
	}
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	C.swe_set_ephe_path(cPath)
}

// Julday returns the Julian day number for the given calendar date and
// fractional UT hour (e.g. 5.5 for 05:30). gregflag selects the calendar:
// CGregCal for Gregorian, 0 for Julian. It is pure calendar arithmetic that
// touches no ephemeris state and is defined for every input, so it never fails
// and may be called from any thread.
func Julday(year, month, day int, hour float64, gregflag int) float64 {
	return float64(C.swe_julday(C.int(year), C.int(month), C.int(day),
		C.double(hour), C.int(gregflag)))
}

// CalcUT computes the position of body at Julian day jdUT (Universal Time)
// under the given flags, on the calling OS thread. It returns the raw
// six-element Swiss Ephemeris result (xx[0] ecliptic longitude in degrees,
// xx[3] longitude speed in degrees per day, the remaining elements latitude,
// distance, and their speeds), the raw C return value, and the Swiss Ephemeris
// error string. A return value below zero signals a rejected request and the
// error string carries the reason; a non-negative return value is success, on
// which the error string may still carry a non-fatal warning. CalcUT does not
// itself turn a failure into a Go error — it reports the raw outcome for the
// caller to gate on. It must run on the thread SetEphePath was called on.
func CalcUT(jdUT float64, body, flags int) (xx [6]float64, retval int, serr string) {
	var cxx [6]C.double
	var cserr [cSerrSize]C.char
	rv := C.swe_calc_ut(C.double(jdUT), C.int(body), C.int(flags),
		&cxx[0], &cserr[0])
	for i := range xx {
		xx[i] = float64(cxx[i])
	}
	return xx, int(rv), C.GoString(&cserr[0])
}

// Houses computes the house cusps and the chart angles for Julian day jdUT
// (Universal Time) at geographic latitude geolat and longitude geolon
// (degrees, north and east positive) under house system hsys (e.g.
// CHouseSystemPlacidus), on the calling OS thread. It returns the raw
// thirteen-element cusp array (index 0 unused, indices 1..12 the twelve house
// cusps in ecliptic-longitude degrees), the raw ten-element angle array
// (ascmc[0] Ascendant, ascmc[1] Midheaven, the rest further points), and the
// raw C return value — below zero on a rejected request. Only house systems of
// at most twelve cusps are in scope; the Gauquelin system, which yields
// thirty-six, is not. It must run on the thread SetEphePath was called on.
func Houses(jdUT float64, geolat, geolon float64, hsys int) (cusps [13]float64, ascmc [10]float64, retval int) {
	// A 37-element cusp buffer is given to C so no house system can overrun it
	// (Gauquelin writes 37); only the first thirteen are returned, the range
	// the in-scope systems fill.
	var ccusps [37]C.double
	var cascmc [10]C.double
	rv := C.swe_houses(C.double(jdUT), C.double(geolat), C.double(geolon),
		C.int(hsys), &ccusps[0], &cascmc[0])
	for i := range cusps {
		cusps[i] = float64(ccusps[i])
	}
	for i := range ascmc {
		ascmc[i] = float64(cascmc[i])
	}
	return cusps, ascmc, int(rv)
}

// HousesARMC computes the house cusps and the chart angles from a right
// ascension of the Midheaven armc (degrees) and the true ecliptic obliquity eps
// (degrees) at geographic latitude geolat (degrees, north positive) under house
// system hsys (e.g. CHouseSystemPlacidus), on the calling OS thread. It is the
// ascensional counterpart of Houses: where Houses takes a Julian day and
// geographic longitude, this takes the already-computed RAMC and obliquity, the
// form a directed or progressed Midheaven supplies. It returns the raw
// thirteen-element cusp array (index 0 unused, indices 1..12 the twelve house
// cusps in ecliptic-longitude degrees), the raw ten-element angle array
// (ascmc[0] Ascendant, ascmc[1] Midheaven, the rest further points), and the
// raw C return value — below zero on a rejected request. Only house systems of
// at most twelve cusps are in scope; the Gauquelin system, which yields
// thirty-six, is not. It must run on the thread SetEphePath was called on.
func HousesARMC(armc, geolat, eps float64, hsys int) (cusps [13]float64, ascmc [10]float64, retval int) {
	// A 37-element cusp buffer is given to C so no house system can overrun it
	// (Gauquelin writes 37); only the first thirteen are returned, the range
	// the in-scope systems fill.
	var ccusps [37]C.double
	var cascmc [10]C.double
	rv := C.swe_houses_armc(C.double(armc), C.double(geolat), C.double(eps),
		C.int(hsys), &ccusps[0], &cascmc[0])
	for i := range cusps {
		cusps[i] = float64(ccusps[i])
	}
	for i := range ascmc {
		ascmc[i] = float64(cascmc[i])
	}
	return cusps, ascmc, int(rv)
}

// Close releases the Swiss Ephemeris resources — the open .se1 file handles
// included — held in the calling OS thread's thread-local state. It must run on
// the thread SetEphePath was called on, so the handles that thread opened are
// the ones freed. After Close, the path must be set again before another CalcUT
// or Houses call on that thread.
func Close() {
	C.swe_close()
}
