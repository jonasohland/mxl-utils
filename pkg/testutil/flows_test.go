package testutil_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jonasohland/mxl-utils/pkg/mxl"
	"github.com/jonasohland/mxl-utils/pkg/testutil"
	"github.com/stretchr/testify/require"
)

const testFlowID = "ae8b77bd-a796-4aed-90ad-57a7dc388f85"

func TestNewVideoFlowDef(t *testing.T) {
	for _, tc := range []struct {
		name   string
		size   testutil.FlowSize
		rate   testutil.FlowRate
		width  int
		height int
		grain  mxl.Rational
	}{
		{name: "1080p25", size: testutil.FlowSize1080, rate: testutil.FlowRate25, width: 1920, height: 1080, grain: mxl.Rational{Numerator: 25, Denominator: 1}},
		{name: "1080p50", size: testutil.FlowSize1080, rate: testutil.FlowRate50, width: 1920, height: 1080, grain: mxl.Rational{Numerator: 50, Denominator: 1}},
		{name: "2160p23.98", size: testutil.FlowSize2160, rate: testutil.FlowRate23, width: 3840, height: 2160, grain: mxl.Rational{Numerator: 24000, Denominator: 1001}},
		{name: "2160p59.94", size: testutil.FlowSize2160, rate: testutil.FlowRate59, width: 3840, height: 2160, grain: mxl.Rational{Numerator: 60000, Denominator: 1001}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			def := testutil.NewVideoFlowDef(tc.size, tc.rate)

			require.Equal(t, "urn:x-nmos:format:video", def.Format)
			require.Equal(t, "video/v210", def.MediaType)
			require.Equal(t, "progressive", def.InterlaceMode)
			require.Equal(t, tc.width, *def.FrameWidth)
			require.Equal(t, tc.height, *def.FrameHeight)
			require.Equal(t, &tc.grain, def.GrainRate)
			require.Nil(t, def.SampleRate)

			_, err := uuid.Parse(def.ID)
			require.NoError(t, err)
		})
	}
}

func TestNewAudioFlowDef(t *testing.T) {
	def := testutil.NewAudioFlowDef()

	require.Equal(t, "urn:x-nmos:format:audio", def.Format)
	require.Equal(t, "audio/float32", def.MediaType)
	require.Equal(t, &mxl.Rational{Numerator: 48000, Denominator: 1}, def.SampleRate)
	require.Equal(t, 64, *def.ChannelCount)
	require.Nil(t, def.GrainRate)

	_, err := uuid.Parse(def.ID)
	require.NoError(t, err)
}

// The generated flow definitions carry a group hint that the flow definition
// parser understands.
func TestGeneratedFlowsHaveGroupHints(t *testing.T) {
	for _, tc := range []struct {
		name string
		def  *mxl.FlowDefinition
		role string
	}{
		{name: "video", def: testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate50), role: "video"},
		{name: "audio", def: testutil.NewAudioFlowDef(), role: "audio"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hint, err := tc.def.GetGroupHint()
			require.NoError(t, err)
			require.Equal(t, tc.def.Label, hint.Name)
			require.Equal(t, tc.role, hint.Type)
		})
	}
}

// Generated flows are distinct from each other, so that several of them can
// live in the same domain.
func TestGeneratedFlowsAreUnique(t *testing.T) {
	domain := t.TempDir()

	first, err := testutil.RandomVideoFlow(domain)
	require.NoError(t, err)
	second, err := testutil.RandomVideoFlow(domain)
	require.NoError(t, err)

	require.NotEqual(t, first.ID(), second.ID())
	require.NotEqual(t, first.Definition().Label, second.Definition().Label)

	require.NoError(t, first.Create())
	require.NoError(t, second.Create())
	require.NotEqual(t, first.Dir(), second.Dir())
}
