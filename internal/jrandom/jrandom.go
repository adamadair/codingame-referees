package jrandom

import "math"

const (
	multiplier = 0x5DEECE66D
	addend     = 0xB
	mask       = (1 << 48) - 1
)

// Random replicates java.util.Random.
type Random struct {
	seed         int64
	nextGaussian float64
	haveGaussian bool
}

func New(seed int64) *Random {
	r := &Random{}
	r.SetSeed(seed)
	return r
}

func (r *Random) SetSeed(seed int64) {
	r.seed = (seed ^ multiplier) & mask
	r.haveGaussian = false
}

func (r *Random) next(bits uint) int32 {
	r.seed = (r.seed*multiplier + addend) & mask
	return int32(r.seed >> (48 - bits)) // arithmetic shift of a 48-bit value, then truncate to int32
}

func (r *Random) NextInt() int32 { return r.next(32) }

// NextIntN matches nextInt(bound). Panics if bound <= 0, like Java throws.
func (r *Random) NextIntN(bound int32) int32 {
	if bound <= 0 {
		panic("bound must be positive")
	}
	rr := r.next(31)
	m := bound - 1
	if bound&m == 0 { // power of two
		return int32((int64(bound) * int64(rr)) >> 31)
	}
	u := rr
	for {
		rr = u % bound
		if u-rr+m >= 0 { // int32 overflow check, wraps like Java
			return rr
		}
		u = r.next(31)
	}
}

func (r *Random) NextLong() int64 {
	return int64(r.next(32))<<32 + int64(r.next(32))
}

func (r *Random) NextBoolean() bool { return r.next(1) != 0 }

func (r *Random) NextDouble() float64 {
	return float64(int64(r.next(26))<<27+int64(r.next(27))) * (1.0 / float64(int64(1)<<53))
}

func (r *Random) NextFloat() float32 {
	return float32(r.next(24)) / float32(1<<24)
}

func (r *Random) NextGaussian() float64 {
	if r.haveGaussian {
		r.haveGaussian = false
		return r.nextGaussian
	}
	for {
		v1 := 2*r.NextDouble() - 1
		v2 := 2*r.NextDouble() - 1
		s := v1*v1 + v2*v2
		if s < 1 && s != 0 {
			mul := math.Sqrt(-2 * math.Log(s) / s) // Java uses StrictMath; may differ in last ulp
			r.nextGaussian = v2 * mul
			r.haveGaussian = true
			return v1 * mul
		}
	}
}
