package jmath

// Atan2 and atan below are ports of fdlibm's e_atan2.c and s_atan.c, which
// Java's Math.atan2 and StrictMath.atan2 use. Go's math.Atan2 uses a
// different algorithm and can differ in the last bit.
//
// Every product that feeds an addition is wrapped in float64(...): an
// explicit conversion stops the Go compiler from fusing it into an FMA
// instruction, which would change the rounding.
//
// The original code carries this notice:
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunSoft, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================

import "math"

func fromBits(hi, lo uint32) float64 { return math.Float64frombits(uint64(hi)<<32 | uint64(lo)) }

func hiWord(x float64) int32  { return int32(math.Float64bits(x) >> 32) }
func loWord(x float64) uint32 { return uint32(math.Float64bits(x)) }

var (
	atanhi = [4]float64{
		fromBits(0x3FDDAC67, 0x0561BB4F), // atan(0.5)hi
		fromBits(0x3FE921FB, 0x54442D18), // atan(1.0)hi
		fromBits(0x3FEF730B, 0xD281F69B), // atan(1.5)hi
		fromBits(0x3FF921FB, 0x54442D18), // atan(inf)hi
	}
	atanlo = [4]float64{
		fromBits(0x3C7A2B7F, 0x222F65E2), // atan(0.5)lo
		fromBits(0x3C81A626, 0x33145C07), // atan(1.0)lo
		fromBits(0x3C700788, 0x7AF0CBBD), // atan(1.5)lo
		fromBits(0x3C91A626, 0x33145C07), // atan(inf)lo
	}
	aT = [11]float64{
		fromBits(0x3FD55555, 0x5555550D),
		fromBits(0xBFC99999, 0x9998EBC4),
		fromBits(0x3FC24924, 0x920083FF),
		fromBits(0xBFBC71C6, 0xFE231671),
		fromBits(0x3FB745CD, 0xC54C206E),
		fromBits(0xBFB3B0F2, 0xAF749A6D),
		fromBits(0x3FB10D66, 0xA0D03D51),
		fromBits(0xBFADDE2D, 0x52DEFD9A),
		fromBits(0x3FA97B4B, 0x24760DEB),
		fromBits(0xBFA2B444, 0x2C6A6C2F),
		fromBits(0x3F90AD3A, 0xE322DA11),
	}

	piO4 = fromBits(0x3FE921FB, 0x54442D18)
	piO2 = fromBits(0x3FF921FB, 0x54442D18)
	pi   = fromBits(0x400921FB, 0x54442D18)
	piLo = fromBits(0x3CA1A626, 0x33145C07)
	tiny = 1.0e-300
	huge = 1.0e300
	one  = 1.0
	negZ = math.Copysign(0, -1)
)

// atan is fdlibm's atan(x).
func atan(x float64) float64 {
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	var id int
	if ix >= 0x44100000 { // |x| >= 2^66
		if ix > 0x7ff00000 || (ix == 0x7ff00000 && loWord(x) != 0) {
			return x + x // NaN
		}
		if hx > 0 {
			return atanhi[3] + atanlo[3]
		}
		return -atanhi[3] - atanlo[3]
	}
	if ix < 0x3fdc0000 { // |x| < 0.4375
		if ix < 0x3e200000 { // |x| < 2^-29
			if huge+x > one {
				return x // raise inexact
			}
		}
		id = -1
	} else {
		x = math.Abs(x)
		if ix < 0x3ff30000 { // |x| < 1.1875
			if ix < 0x3fe60000 { // 7/16 <= |x| < 11/16
				id = 0
				x = (float64(2.0*x) - one) / (2.0 + x)
			} else { // 11/16 <= |x| < 19/16
				id = 1
				x = (x - one) / (x + one)
			}
		} else {
			if ix < 0x40038000 { // |x| < 2.4375
				id = 2
				x = (x - 1.5) / (one + float64(1.5*x))
			} else { // 2.4375 <= |x| < 2^66
				id = 3
				x = -1.0 / x
			}
		}
	}
	// end of argument reduction
	z := float64(x * x)
	w := float64(z * z)
	// break sum from i=0 to 10 aT[i]z**(i+1) into odd and even poly
	// s1 = z*(aT[0]+w*(aT[2]+w*(aT[4]+w*(aT[6]+w*(aT[8]+w*aT[10])))))
	s1 := aT[8] + float64(w*aT[10])
	s1 = aT[6] + float64(w*s1)
	s1 = aT[4] + float64(w*s1)
	s1 = aT[2] + float64(w*s1)
	s1 = aT[0] + float64(w*s1)
	s1 = float64(z * s1)
	// s2 = w*(aT[1]+w*(aT[3]+w*(aT[5]+w*(aT[7]+w*aT[9]))))
	s2 := aT[7] + float64(w*aT[9])
	s2 = aT[5] + float64(w*s2)
	s2 = aT[3] + float64(w*s2)
	s2 = aT[1] + float64(w*s2)
	s2 = float64(w * s2)
	if id < 0 {
		return x - float64(x*(s1+s2))
	}
	z = atanhi[id] - ((float64(x*(s1+s2)) - atanlo[id]) - x)
	if hx < 0 {
		return -z
	}
	return z
}

// Atan2 is Java's Math.atan2(y, x), bit for bit.
func Atan2(y, x float64) float64 {
	hx := hiWord(x)
	ix := hx & 0x7fffffff
	lx := loWord(x)
	hy := hiWord(y)
	iy := hy & 0x7fffffff
	ly := loWord(y)
	if math.IsNaN(x) || math.IsNaN(y) {
		return x + y
	}
	if (hx-0x3ff00000)|int32(lx) == 0 { // x = 1.0
		return atan(y)
	}
	m := (hy>>31)&1 | (hx>>30)&2 // 2*sign(x)+sign(y)

	// when y = 0
	if iy|int32(ly) == 0 {
		switch m {
		case 0, 1:
			return y // atan(+-0, +anything) = +-0
		case 2:
			return pi + tiny // atan(+0, -anything) = pi
		case 3:
			return -pi - tiny // atan(-0, -anything) = -pi
		}
	}
	// when x = 0
	if ix|int32(lx) == 0 {
		if hy < 0 {
			return -piO2 - tiny
		}
		return piO2 + tiny
	}
	// when x is INF
	if ix == 0x7ff00000 {
		if iy == 0x7ff00000 {
			switch m {
			case 0:
				return piO4 + tiny // atan(+INF, +INF)
			case 1:
				return -piO4 - tiny // atan(-INF, +INF)
			case 2:
				return float64(3.0*piO4) + tiny // atan(+INF, -INF)
			default:
				return float64(-3.0*piO4) - tiny // atan(-INF, -INF)
			}
		}
		switch m {
		case 0:
			return 0 // atan(+..., +INF)
		case 1:
			return negZ // atan(-..., +INF)
		case 2:
			return pi + tiny // atan(+..., -INF)
		default:
			return -pi - tiny // atan(-..., -INF)
		}
	}
	// when y is INF
	if iy == 0x7ff00000 {
		if hy < 0 {
			return -piO2 - tiny
		}
		return piO2 + tiny
	}

	// compute y/x
	var z float64
	k := (iy - ix) >> 20
	switch {
	case k > 60: // |y/x| > 2**60
		z = piO2 + float64(0.5*piLo)
	case hx < 0 && k < -60: // |y|/x < -2**60
		z = 0.0
	default:
		z = atan(math.Abs(y / x)) // safe to do y/x
	}
	switch m {
	case 0:
		return z // atan(+, +)
	case 1:
		return math.Float64frombits(math.Float64bits(z) ^ 1<<63) // atan(-, +)
	case 2:
		return pi - (z - piLo) // atan(+, -)
	default:
		return (z - piLo) - pi // atan(-, -)
	}
}
