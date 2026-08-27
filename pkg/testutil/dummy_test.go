package testutil_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jonasohland/mxl-utils/pkg/mxl"
	"github.com/jonasohland/mxl-utils/pkg/mxlsys"
	"github.com/jonasohland/mxl-utils/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func TestDummyFlowCreateAndRemove(t *testing.T) {
	domain := t.TempDir()

	flow, err := testutil.RandomVideoFlow(domain)
	require.NoError(t, err)
	require.Equal(t, domain, flow.Domain())
	require.Empty(t, flow.Dir(), "the flow directory only exists once the flow was created")

	require.NoError(t, flow.Create())
	require.Equal(t, filepath.Join(domain, flow.ID()+".mxl-flow"), flow.Dir())
	require.FileExists(t, filepath.Join(flow.Dir(), "data"))
	require.FileExists(t, filepath.Join(flow.Dir(), "flow_def.json"))

	// The flow was moved into place, no temporary directory is left behind
	entries, err := os.ReadDir(domain)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, flow.Remove())
	require.NoDirExists(t, flow.Dir())
}

// The data file must have exactly the size of the flow info structure, since
// that is what readers map.
func TestDummyFlowDataFileSize(t *testing.T) {
	domain := t.TempDir()

	for _, tc := range []struct {
		name string
		flow func(string) (*testutil.DummyFlow, error)
	}{
		{name: "video", flow: testutil.RandomVideoFlow},
		{name: "audio", flow: testutil.RandomAudioFlow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flow, err := tc.flow(domain)
			require.NoError(t, err)
			require.NoError(t, flow.Create())

			stat, err := os.Stat(filepath.Join(flow.Dir(), "data"))
			require.NoError(t, err)
			require.EqualValues(t, 2048, stat.Size())
		})
	}
}

// Audio flows carry a continuous config, which used to be written as a zeroed
// block because the discrete config was encoded instead.
func TestDummyAudioFlowWritesContinuousConfig(t *testing.T) {
	domain := t.TempDir()

	flow, err := testutil.RandomAudioFlow(domain)
	require.NoError(t, err)
	require.NoError(t, flow.Create())

	opened, err := mxl.Open(domain, flow.ID())
	require.NoError(t, err)
	defer func() { _ = opened.Close() }()

	config, _, err := opened.GetInfo()
	require.NoError(t, err)

	require.Equal(t, mxl.DataFormatAudio, config.Format)
	require.NotNil(t, config.Continuous)
	require.NotZero(t, config.Continuous.Channels)
	require.NotZero(t, config.Continuous.BufferLength)
}

func TestDummyFlowWritesDefinition(t *testing.T) {
	domain := t.TempDir()

	def := testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate25)
	flow, err := testutil.NewDummyFlow(domain, def)
	require.NoError(t, err)
	require.NoError(t, flow.Create())

	fd, err := os.Open(filepath.Join(flow.Dir(), "flow_def.json"))
	require.NoError(t, err)
	defer func() { _ = fd.Close() }()

	written := &mxl.FlowDefinition{}
	require.NoError(t, json.NewDecoder(fd).Decode(written))
	require.Equal(t, def, written)
}

func TestDummyFlowUpdateRuntime(t *testing.T) {
	domain := t.TempDir()

	flow, err := testutil.RandomVideoFlow(domain)
	require.NoError(t, err)

	// A flow that does not exist yet cannot be updated
	require.ErrorIs(t, flow.UpdateRuntime(mxlsys.FlowRuntimeInfo{}), os.ErrNotExist)

	require.NoError(t, flow.Create())

	runtime := mxlsys.FlowRuntimeInfo{HeadIndex: 99, LastWriteTime: 98, LastReadTime: 97}
	require.NoError(t, flow.UpdateRuntime(runtime))
	require.Equal(t, runtime, flow.Runtime())

	stat, err := os.Stat(filepath.Join(flow.Dir(), "data"))
	require.NoError(t, err)
	require.EqualValues(t, 2048, stat.Size(), "the data file must not grow when it is updated")
}

func TestNewDummyFlowErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		def  *mxl.FlowDefinition
		err  error
	}{
		{
			name: "flow id is not a uuid",
			def:  &mxl.FlowDefinition{ID: "not-a-uuid", Format: "urn:x-nmos:format:video", GrainRate: &mxl.Rational{Numerator: 25, Denominator: 1}},
		},
		{
			name: "neither format nor media type are known",
			def:  &mxl.FlowDefinition{ID: testFlowID, MediaType: "video/nonsense"},
			err:  testutil.ErrUnknownFormat,
		},
		{
			name: "flow definition carries no rate",
			def:  &mxl.FlowDefinition{ID: testFlowID, Format: "urn:x-nmos:format:video"},
			err:  testutil.ErrMissingRate,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := testutil.NewDummyFlow(t.TempDir(), tc.def)
			require.Error(t, err)

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			}
		})
	}
}

// The data format follows the NMOS format of the flow definition, and falls
// back to the media type when the definition does not carry one.
func TestDummyFlowDataFormat(t *testing.T) {
	for _, tc := range []struct {
		name      string
		format    string
		mediaType string
		expected  mxl.DataFormat
		discrete  bool
	}{
		{name: "video", format: "urn:x-nmos:format:video", mediaType: "video/v210", expected: mxl.DataFormatVideo, discrete: true},
		{name: "audio", format: "urn:x-nmos:format:audio", mediaType: "audio/float32", expected: mxl.DataFormatAudio},
		{name: "data", format: "urn:x-nmos:format:data", mediaType: "video/smpte291", expected: mxl.DataFormatData, discrete: true},
		{name: "video from media type", mediaType: "video/v210", expected: mxl.DataFormatVideo, discrete: true},
		{name: "audio from media type", mediaType: "audio/float32", expected: mxl.DataFormatAudio},
		{name: "data from media type", mediaType: "video/smpte291", expected: mxl.DataFormatData, discrete: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			domain := t.TempDir()

			channels := 8
			flow, err := testutil.NewDummyFlow(domain, &mxl.FlowDefinition{
				ID:           testFlowID,
				Format:       tc.format,
				MediaType:    tc.mediaType,
				GrainRate:    &mxl.Rational{Numerator: 25, Denominator: 1},
				SampleRate:   &mxl.Rational{Numerator: 48000, Denominator: 1},
				ChannelCount: &channels,
			})
			require.NoError(t, err)
			require.NoError(t, flow.Create())

			opened, err := mxl.Open(domain, flow.ID())
			require.NoError(t, err)
			defer func() { _ = opened.Close() }()

			config, _, err := opened.GetInfo()
			require.NoError(t, err)
			require.Equal(t, tc.expected, config.Format)

			if tc.discrete {
				require.NotNil(t, config.Discrete)
				// Discrete formats are timed by the grain rate
				require.Equal(t, mxl.Rational{Numerator: 25, Denominator: 1}, config.Rate)
			} else {
				require.NotNil(t, config.Continuous)
				// Continuous formats are timed by the sample rate
				require.Equal(t, mxl.Rational{Numerator: 48000, Denominator: 1}, config.Rate)
			}
		})
	}
}
