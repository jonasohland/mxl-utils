package mxl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jonasohland/mxl-utils/pkg/mxl"
	"github.com/jonasohland/mxl-utils/pkg/mxlsys"
	"github.com/jonasohland/mxl-utils/pkg/testutil"
	"github.com/stretchr/testify/require"
)

// Creates a flow from the given definition in a fresh domain directory and
// returns the domain path and the created flow.
func createFlow(t *testing.T, def *mxl.FlowDefinition) (string, *testutil.DummyFlow) {
	t.Helper()

	domain := t.TempDir()
	flow, err := testutil.NewDummyFlow(domain, def)
	require.NoError(t, err)
	require.NoError(t, flow.Create())

	return domain, flow
}

func openFlow(t *testing.T, domain string, dummy *testutil.DummyFlow) *mxl.Flow {
	t.Helper()

	flow, err := mxl.Open(domain, dummy.ID())
	require.NoError(t, err)
	t.Cleanup(func() { _ = flow.Close() })

	return flow
}

func TestOpenVideoFlow(t *testing.T) {
	domain, dummy := createFlow(t, testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate50))
	flow := openFlow(t, domain, dummy)

	config, runtime, err := flow.GetInfo()
	require.NoError(t, err)

	require.Equal(t, dummy.ID(), config.ID)
	require.Equal(t, mxl.DataFormatVideo, config.Format)
	require.Equal(t, mxl.Rational{Numerator: 50, Denominator: 1}, config.Rate)
	require.Equal(t, mxl.PayloadLocationHost, config.PayloadLocation)
	require.Equal(t, -1, config.DeviceIndex)

	// Video is a discrete format, so only the discrete config is populated
	require.Nil(t, config.Continuous)
	require.NotNil(t, config.Discrete)
	require.Equal(t, [4]uint32{1, 0, 0, 0}, config.Discrete.SliceSizes)
	require.Equal(t, uint32(11), config.Discrete.GrainCount)

	require.Equal(t, uint64(10), runtime.HeadIndex)
	require.Equal(t, uint64(10), runtime.LastWriteTime)
	require.Equal(t, uint64(0), runtime.LastReadTime)
}

func TestOpenAudioFlow(t *testing.T) {
	domain, dummy := createFlow(t, testutil.NewAudioFlowDef())
	flow := openFlow(t, domain, dummy)

	config, _, err := flow.GetInfo()
	require.NoError(t, err)

	require.Equal(t, mxl.DataFormatAudio, config.Format)
	// For audio flows the configured rate is the sample rate
	require.Equal(t, mxl.Rational{Numerator: 48000, Denominator: 1}, config.Rate)

	// Audio is a continuous format, so only the continuous config is populated
	require.Nil(t, config.Discrete)
	require.NotNil(t, config.Continuous)
	require.Equal(t, uint32(64), config.Continuous.Channels)
	require.Equal(t, uint32(48000), config.Continuous.BufferLength)
}

func TestOpenDataFlow(t *testing.T) {
	def := testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate25)
	def.Format = "urn:x-nmos:format:data"
	def.MediaType = "video/smpte291"

	domain, dummy := createFlow(t, def)
	flow := openFlow(t, domain, dummy)

	config, _, err := flow.GetInfo()
	require.NoError(t, err)

	// Data flows are discrete, just like video flows
	require.Equal(t, mxl.DataFormatData, config.Format)
	require.NotNil(t, config.Discrete)
	require.Nil(t, config.Continuous)
}

func TestOpenMissingFlow(t *testing.T) {
	_, err := mxl.Open(t.TempDir(), testFlowID)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestOpenTruncatedFlow(t *testing.T) {
	domain, dummy := createFlow(t, testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate50))

	data := filepath.Join(dummy.Dir(), "data")
	require.NoError(t, os.Truncate(data, 32))

	flow := openFlow(t, domain, dummy)

	_, _, err := flow.GetInfo()
	require.Error(t, err)
}

func TestGetDefinition(t *testing.T) {
	def := testutil.NewVideoFlowDef(testutil.FlowSize2160, testutil.FlowRate23)
	domain, dummy := createFlow(t, def)
	flow := openFlow(t, domain, dummy)

	read, err := flow.GetDefinition()
	require.NoError(t, err)

	require.Equal(t, def.ID, read.ID)
	require.Equal(t, def.Label, read.Label)
	require.Equal(t, def.Description, read.Description)
	require.Equal(t, def.MediaType, read.MediaType)
	require.Equal(t, def.Format, read.Format)
	require.Equal(t, def.InterlaceMode, read.InterlaceMode)
	require.Equal(t, 3840, *read.FrameWidth)
	require.Equal(t, 2160, *read.FrameHeight)
	require.Equal(t, &mxl.Rational{Numerator: 24000, Denominator: 1001}, read.GrainRate)
	require.Equal(t, def.Tags, read.Tags)
}

func TestGetDefinitionMissingFile(t *testing.T) {
	domain, dummy := createFlow(t, testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate50))
	flow := openFlow(t, domain, dummy)

	require.NoError(t, os.Remove(filepath.Join(dummy.Dir(), "flow_def.json")))

	_, err := flow.GetDefinition()
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestGetDefinitionInvalidJSON(t *testing.T) {
	domain, dummy := createFlow(t, testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate50))
	flow := openFlow(t, domain, dummy)

	require.NoError(t, os.WriteFile(filepath.Join(dummy.Dir(), "flow_def.json"), []byte("{not json"), 0o0644))

	_, err := flow.GetDefinition()
	require.Error(t, err)
}

// The flow info is read through a shared mapping, so writes by the producer
// must be visible without reopening the flow.
func TestGetInfoObservesRuntimeUpdates(t *testing.T) {
	domain, dummy := createFlow(t, testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate50))
	flow := openFlow(t, domain, dummy)

	_, runtime, err := flow.GetInfo()
	require.NoError(t, err)
	require.Equal(t, uint64(10), runtime.HeadIndex)

	require.NoError(t, dummy.UpdateRuntime(mxlsys.FlowRuntimeInfo{
		HeadIndex:     4711,
		LastWriteTime: 1234,
		LastReadTime:  5678,
	}))

	_, runtime, err = flow.GetInfo()
	require.NoError(t, err)
	require.Equal(t, uint64(4711), runtime.HeadIndex)
	require.Equal(t, uint64(1234), runtime.LastWriteTime)
	require.Equal(t, uint64(5678), runtime.LastReadTime)
}

func TestFlowIsValid(t *testing.T) {
	domain, dummy := createFlow(t, testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate50))

	flow, err := mxl.Open(domain, dummy.ID())
	require.NoError(t, err)
	defer func() { _ = flow.Close() }()

	require.True(t, flow.IsValid())

	// A flow whose data file is gone is not valid anymore
	require.NoError(t, dummy.Remove())
	require.False(t, flow.IsValid())

	// Neither is one that was replaced by a new flow with the same id, since
	// the mapping still refers to the data file of the previous incarnation
	replaced, err := testutil.NewDummyFlow(domain, dummy.Definition())
	require.NoError(t, err)
	require.NoError(t, replaced.Create())
	require.False(t, flow.IsValid())

	// Reopening picks up the new data file
	reopened, err := mxl.Open(domain, dummy.ID())
	require.NoError(t, err)
	defer func() { _ = reopened.Close() }()
	require.True(t, reopened.IsValid())
}

// Every open flow holds a file descriptor for the mapped data file, which must
// be released on close.
func TestCloseReleasesFileDescriptor(t *testing.T) {
	domain, dummy := createFlow(t, testutil.NewVideoFlowDef(testutil.FlowSize1080, testutil.FlowRate50))

	before := openFileDescriptors(t)

	for range 64 {
		flow, err := mxl.Open(domain, dummy.ID())
		require.NoError(t, err)
		require.NoError(t, flow.Close())
	}

	require.Equal(t, before, openFileDescriptors(t))
}

func openFileDescriptors(t *testing.T) int {
	t.Helper()

	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)

	return len(entries)
}
