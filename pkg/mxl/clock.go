package mxl

import (
	"time"

	"github.com/shabbyrobe/go-num"
	"golang.org/x/sys/unix"
)

func Now() (time.Time, error) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_TAI, &ts); err != nil {
		return time.Time{}, err
	}

	return time.Unix(ts.Sec, int64(ts.Nsec)), nil
}

func TimestampToIndex(tm time.Time, rate Rational) uint64 {
	ts := num.I128From64(tm.UnixNano())
	nm := num.I128From64(rate.Numerator)
	dn := num.I128From64(rate.Denominator)

	return (ts.Mul(nm).Add(dn.Mul64(500_000_000))).
		Quo(dn.Mul64(1_000_000_000)).AsUint64()
}

func IndexToTimestamp(index uint64, rate Rational) time.Time {
	i := num.I128FromU64(index)
	nm := num.I128From64(rate.Numerator)
	dn := num.I128From64(rate.Denominator)

	ts := i.Mul(dn).Mul64(1_000_000_000).Add(nm.Quo64(2)).Quo(nm).AsInt64()
	return time.Unix(0, ts)
}

func CurrentIndex(rate Rational) uint64 {
	now, err := Now()
	if err != nil {
		return 0
	}

	return TimestampToIndex(now, rate)
}
