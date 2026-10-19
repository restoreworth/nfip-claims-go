// Package nfipclaims gives, for each US state and the District of Columbia,
// the distribution of the building amounts paid by the National Flood
// Insurance Program (NFIP) on claims with a year of loss in 2020-2024, and
// places an amount within that distribution.
//
// These are amounts paid by NFIP, not repair cost. They are what NFIP paid the
// policyholder on the building, after deductible and capped by the coverage
// purchased; uninsured, undeclared or non-covered damage is not included. They
// cover only NFIP-insured properties with a paid claim and are not
// representative of all homes or all floods. Amounts are nominal dollars of
// different years and of different disasters, except Median2024USD.
//
// # Source
//
// The statistics are computed from FEMA's OpenFEMA dataset NFIP Redacted
// Claims, version v3, retrieved from the API endpoint:
//
// https://www.fema.gov/api/open/v3/NfipClaims
//
// The asOfDate of the records is 2026-09-08 (constant AsOfDate); the
// retrieval time is the constant RetrievedUTC.
//
// This product uses the Federal Emergency Management Agency’s OpenFEMA API, but is not endorsed by FEMA. The Federal Government or FEMA cannot vouch for the data or analyses derived from these data after the data have been retrieved from the Agency's website(s).
//
// This package is not an official federal report. OpenFEMA terms and
// conditions:
//
// https://www.fema.gov/about/openfema/terms-conditions
//
// # Method
//
// A claim is paid when amountPaidOnBuildingClaim > 0. Quartiles are
// Hyndman-Fan type 7 (linear interpolation) on the paid amounts, rounded to
// the cent. A state with fewer than 100 paid claims has no statistics and the
// status "insufficient". The aggregation code and the full method are in the
// nfip-paid-claims-by-state dataset; nfip_gen.go is generated from its JSON
// file by tools/gen_go.py.
//
// Maintained by the RestoreWorth Team:
//
// https://restoreworth.com/
//
// The same table, with notes, on the site:
//
// https://restoreworth.com/nfip-flood-claims-by-state.html
//
// # Licence
//
// Code: MIT License (LICENSE). The aggregates (nfip_gen.go, testdata/) are also available under CC BY 4.0, attribution "RestoreWorth Team" (DATA_NOTICE.md). The underlying FEMA data are subject to the OpenFEMA terms and conditions.
package nfipclaims

import (
	"math"
	"strings"
)

// StatusOK and StatusInsufficient are the values of StateStats.Status.
const (
	StatusOK           = "ok"
	StatusInsufficient = "insufficient"
)

// StateStats is the record of one state. When Status is StatusInsufficient,
// Q1, Median, Q3, TotalPaid, Median2024USD, TopEvent, TopEventShare,
// MedianExclTopEvent, HasMedianExclTopEvent and EventDependent are zero.
type StateStats struct {
	// State is the two-letter USPS code ("DC" for the District of Columbia).
	State string
	// Window is the range of years of loss, "2020-2024".
	Window string
	// AsOfDate and RetrievedUTC date the source records.
	AsOfDate, RetrievedUTC string
	// NClaims counts every claim record of the state in the window, NPaid the
	// paid ones.
	NClaims, NPaid int
	// Q1, Median and Q3 are the quartiles of the paid amounts, in US dollars.
	Q1, Median, Q3 float64
	// TotalPaid is the sum of the paid amounts, in US dollars.
	TotalPaid float64
	// Median2024USD is the median after each amount is multiplied by
	// CPI-U(2024) / CPI-U(year of loss): a monetary adjustment, not today's
	// cost of repair.
	Median2024USD float64
	// TopEvent is the named flood event with the most paid claims in the
	// state, "" if none; TopEventShare is its share of the paid claims (0 if
	// none).
	TopEvent      string
	TopEventShare float64
	// MedianExclTopEvent is the median of the paid claims not in TopEvent. It
	// is set only when HasMedianExclTopEvent is true (100 claims or more
	// remain).
	MedianExclTopEvent    float64
	HasMedianExclTopEvent bool
	// EventDependent is true when MedianExclTopEvent differs from Median by
	// more than 25 %. It is meaningful only when HasMedianExclTopEvent is
	// true.
	EventDependent bool
	// Status is StatusOK or StatusInsufficient.
	Status string
}

// States returns the 51 records, sorted by State. The slice is a copy: the
// caller may modify it.
func States() []StateStats {
	return append([]StateStats(nil), states...)
}

// Lookup returns the record of a state, given its two-letter code (case and
// surrounding spaces ignored). The boolean is false for an unknown code.
func Lookup(state string) (StateStats, bool) {
	code := strings.ToUpper(strings.TrimSpace(state))
	for _, s := range states {
		if s.State == code {
			return s, true
		}
	}
	return StateStats{}, false
}

// Position returns the quartile, 1 to 4, of an amount in US dollars among the
// paid amounts of a state: 1 up to Q1, 2 up to Median, 3 up to Q3, 4 above
// Q3 (a bound belongs to the lower quartile). ok is false when the state is
// unknown, its Status is StatusInsufficient, or the amount is NaN.
func Position(state string, amountUSD float64) (quartile int, ok bool) {
	s, found := Lookup(state)
	if !found || s.Status != StatusOK || math.IsNaN(amountUSD) {
		return 0, false
	}
	switch {
	case amountUSD <= s.Q1:
		return 1, true
	case amountUSD <= s.Median:
		return 2, true
	case amountUSD <= s.Q3:
		return 3, true
	}
	return 4, true
}
