package mxl_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jonasohland/mxl-utils/pkg/mxl"
	"github.com/stretchr/testify/require"
)

var (
	rate25   = mxl.Rational{Numerator: 25, Denominator: 1}
	rate50   = mxl.Rational{Numerator: 50, Denominator: 1}
	rate2398 = mxl.Rational{Numerator: 24000, Denominator: 1001}
	rate5994 = mxl.Rational{Numerator: 60000, Denominator: 1001}
	rate48k  = mxl.Rational{Numerator: 48000, Denominator: 1}
)

func TestTimestampToIndex(t *testing.T) {
	for _, tc := range []struct {
		name  string
		nanos int64
		rate  mxl.Rational
		index uint64
	}{
		{name: "epoch", nanos: 0, rate: rate25, index: 0},
		{name: "one second at 25", nanos: int64(time.Second), rate: rate25, index: 25},
		{name: "one second at 50", nanos: int64(time.Second), rate: rate50, index: 50},
		{name: "one second at 48k", nanos: int64(time.Second), rate: rate48k, index: 48000},
		{name: "one grain at 25", nanos: 40_000_000, rate: rate25, index: 1},
		{name: "just below a grain boundary", nanos: 39_999_999, rate: rate25, index: 1},
		{name: "rounds half up", nanos: 20_000_000, rate: rate25, index: 1},
		{name: "rounds down below half", nanos: 19_999_999, rate: rate25, index: 0},
		{name: "one second at 23.98", nanos: int64(time.Second), rate: rate2398, index: 24},
		{name: "one second at 59.94", nanos: int64(time.Second), rate: rate5994, index: 60},
		{name: "an hour at 59.94", nanos: int64(time.Hour), rate: rate5994, index: 215_784},
		// A realistic TAI timestamp, well past the range of an int32
		{name: "far past the epoch", nanos: 1_700_000_000_000_000_000, rate: rate25, index: 42_500_000_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.index, mxl.TimestampToIndex(time.Unix(0, tc.nanos), tc.rate))
		})
	}
}

func TestIndexToTimestamp(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index uint64
		rate  mxl.Rational
		nanos int64
	}{
		{name: "epoch", index: 0, rate: rate25, nanos: 0},
		{name: "one second at 25", index: 25, rate: rate25, nanos: int64(time.Second)},
		{name: "one second at 50", index: 50, rate: rate50, nanos: int64(time.Second)},
		{name: "one second at 48k", index: 48000, rate: rate48k, nanos: int64(time.Second)},
		{name: "one grain at 25", index: 1, rate: rate25, nanos: 40_000_000},
		{name: "one grain at 23.98", index: 1, rate: rate2398, nanos: 41_708_333},
		{name: "far past the epoch", index: 42_500_000_000, rate: rate25, nanos: 1_700_000_000_000_000_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, time.Unix(0, tc.nanos), mxl.IndexToTimestamp(tc.index, tc.rate))
		})
	}
}

// Converting an index to a timestamp and back must be lossless for every
// supported rate, including the fractional ones.
func TestIndexTimestampRoundTrip(t *testing.T) {
	for _, rate := range []mxl.Rational{rate25, rate50, rate2398, rate5994, rate48k} {
		t.Run(rateName(rate), func(t *testing.T) {
			for _, index := range []uint64{0, 1, 2, 25, 1000, 123_457, 42_500_000_000} {
				ts := mxl.IndexToTimestamp(index, rate)
				require.Equal(t, index, mxl.TimestampToIndex(ts, rate), "index %d, timestamp %v", index, ts)
			}
		})
	}
}

func TestNow(t *testing.T) {
	now, err := mxl.Now()
	require.NoError(t, err)
	require.False(t, now.IsZero())

	// TAI runs ahead of UTC by a fixed offset (37 seconds as of 2026), and is
	// equal to it on hosts that have no offset configured.
	offset := now.Sub(time.Now())
	require.GreaterOrEqual(t, offset, -2*time.Second)
	require.LessOrEqual(t, offset, 2*time.Minute)
}

func TestCurrentIndex(t *testing.T) {
	now, err := mxl.Now()
	require.NoError(t, err)

	expected := mxl.TimestampToIndex(now, rate50)
	index := mxl.CurrentIndex(rate50)

	// Both are sampled from the same clock a moment apart
	require.InDelta(t, expected, index, 50)
	require.GreaterOrEqual(t, index, expected)
}

func rateName(rate mxl.Rational) string {
	return fmt.Sprintf("%d/%d", rate.Numerator, rate.Denominator)
}
