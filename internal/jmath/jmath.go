// Package jmath reproduces java.lang.Math results that Go's math package
// computes differently: atan2 (Java uses fdlibm), Math.round, and the
// (int) and (long) casts from double.
package jmath

import "math"

// Round is Java's Math.round(double): round half up, so Round(-2.5) is -2
// (Go's math.Round gives -3). NaN gives 0 and out-of-range values
// saturate, as Java's (long) cast does. This is a port of the JDK's
// bit-level implementation.
func Round(a float64) int64 {
	const (
		significandWidth = 53
		expBias          = 1023
		expBitMask       = 0x7FF0000000000000
		signifBitMask    = 0x000FFFFFFFFFFFFF
	)
	bits := int64(math.Float64bits(a))
	biasedExp := (bits & expBitMask) >> (significandWidth - 1)
	shift := (significandWidth - 2 + expBias) - biasedExp
	if shift&-64 == 0 { // 0 <= shift < 64
		r := (bits & signifBitMask) | (signifBitMask + 1)
		if bits < 0 {
			r = -r
		}
		return ((r >> shift) + 1) >> 1
	}
	return Long(a)
}

// Long is Java's (long) cast from double: truncate toward zero, NaN gives
// 0, and values outside the int64 range saturate. Go leaves those cases
// undefined.
func Long(a float64) int64 {
	switch {
	case math.IsNaN(a):
		return 0
	case a >= math.MaxInt64: // 2^63 rounds up, so >= catches it
		return math.MaxInt64
	case a <= math.MinInt64:
		return math.MinInt64
	}
	return int64(a)
}

// Int is Java's (int) cast from double.
func Int(a float64) int32 {
	switch {
	case math.IsNaN(a):
		return 0
	case a >= math.MaxInt32:
		return math.MaxInt32
	case a <= math.MinInt32:
		return math.MinInt32
	}
	return int32(a)
}
