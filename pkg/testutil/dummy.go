package testutil

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jonasohland/mxl-utils/pkg/mxl"
	"github.com/jonasohland/mxl-utils/pkg/mxlsys"
	"github.com/samber/lo"
)

var (
	ErrUnknownFormat = errors.New("cannot determine data format for flow")
	ErrMissingRate   = errors.New("flow definition has no grain rate or sample rate")
)

// Data format values as defined by mxlDataFormat in <mxl/dataformat.h>
const (
	formatUnspec uint32 = 0
	formatVideo  uint32 = 1
	formatAudio  uint32 = 2
	formatData   uint32 = 3
)

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// Translates the NMOS IS-04 flow format to a data format, the same way the MXL
// library does. Falls back to the media type for flow definitions that do not
// carry a format.
func dataFormat(flowDef *mxl.FlowDefinition) uint32 {
	switch flowDef.Format {
	case "urn:x-nmos:format:video":
		return formatVideo
	case "urn:x-nmos:format:audio":
		return formatAudio
	case "urn:x-nmos:format:data":
		return formatData
	}

	switch flowDef.MediaType {
	case "video/v210":
		return formatVideo
	case "video/smpte291":
		return formatData
	case "audio/float32":
		return formatAudio
	}

	return formatUnspec
}

// The rate stored in the flow info. For video and data flows this is the grain
// rate, for audio flows the sample rate.
func configuredRate(flowDef *mxl.FlowDefinition, format uint32) (mxlsys.Rational, error) {
	rate := lo.Ternary(format == formatAudio, flowDef.SampleRate, flowDef.GrainRate)
	if rate == nil {
		// Accept a flow definition that only carries the other kind of rate,
		// so that partially populated definitions remain usable in tests.
		rate = lo.CoalesceOrEmpty(flowDef.GrainRate, flowDef.SampleRate)
		if rate == nil {
			return mxlsys.Rational{}, ErrMissingRate
		}
	}

	return mxlsys.Rational{Numerator: rate.Numerator, Denominator: rate.Denominator}, nil
}

type DummyFlow struct {
	flowDef              *mxl.FlowDefinition
	flowInfo             *mxlsys.FlowInfo
	flowConfigDiscrete   *mxlsys.DiscreteFlowConfigInfo
	flowConfigContinuous *mxlsys.ContinuousFlowConfigInfo
	domain               string
	flowDir              string
}

func NewDummyFlow(domain string, flowDef *mxl.FlowDefinition) (*DummyFlow, error) {
	id, err := uuid.Parse(flowDef.ID)
	if err != nil {
		return nil, err
	}

	format := dataFormat(flowDef)
	if format == formatUnspec {
		return nil, fmt.Errorf("%w: format %q, media type %q", ErrUnknownFormat, flowDef.Format, flowDef.MediaType)
	}

	rate, err := configuredRate(flowDef, format)
	if err != nil {
		return nil, err
	}

	flow := &DummyFlow{
		flowDef: flowDef,
		flowInfo: &mxlsys.FlowInfo{
			Version: 1,
			Size:    2048,
			Config: mxlsys.FlowConfigInfo{
				Common: mxlsys.CommonFlowConfigInfo{
					ID:                     id,
					Format:                 format,
					Flags:                  0,
					Rate:                   rate,
					MaxCommitBatchSizeHint: 1,
					MaxSyncBatchSizeHint:   1,
					PayloadLocation:        0,
					DeviceIndex:            -1,
				},
			},
			Runtime: mxlsys.FlowRuntimeInfo{
				HeadIndex:     10,
				LastWriteTime: 10,
				LastReadTime:  0,
			},
		},
		domain:  domain,
		flowDir: "",
	}

	if format == formatAudio {
		flow.flowConfigContinuous = &mxlsys.ContinuousFlowConfigInfo{
			Channels:     lo.Ternary(flowDef.ChannelCount == nil, 0, uint32(lo.FromPtr(flowDef.ChannelCount))),
			BufferLength: 48000,
		}
	} else {
		flow.flowConfigDiscrete = &mxlsys.DiscreteFlowConfigInfo{
			SliceSizes: [4]uint32{1, 0, 0, 0},
			GrainCount: 11,
		}
	}

	return flow, nil
}

// The flow id, as it appears in the name of the flow directory.
func (d *DummyFlow) ID() string {
	return d.flowDef.ID
}

// The flow definition this flow was created from.
func (d *DummyFlow) Definition() *mxl.FlowDefinition {
	return d.flowDef
}

// The domain directory this flow lives in.
func (d *DummyFlow) Domain() string {
	return d.domain
}

// The flow directory. Empty until the flow has been created, and points at the
// temporary directory while the flow is being created.
func (d *DummyFlow) Dir() string {
	return d.flowDir
}

// The runtime info the flow was created with, or last updated to.
func (d *DummyFlow) Runtime() mxlsys.FlowRuntimeInfo {
	return d.flowInfo.Runtime
}

func (d *DummyFlow) createTmpFlowDir() error {
	tmpFlowDir := filepath.Join(d.domain, ".tmp-"+generateRandomString(16))
	if err := os.MkdirAll(tmpFlowDir, 0o0755); err != nil {
		return err
	}

	d.flowDir = tmpFlowDir
	return nil
}

func (d *DummyFlow) writeFlowDef() error {
	fd, err := os.OpenFile(filepath.Join(d.flowDir, "flow_def.json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o0644)
	if err != nil {
		return err
	}
	defer func() { _ = fd.Close() }()

	if err := json.NewEncoder(fd).Encode(d.flowDef); err != nil {
		return err
	}

	return nil
}

// Encodes the format specific part of the flow config into the fixed size
// union that follows the common config in the flow info.
func (d *DummyFlow) encodeTypedConfig() error {
	typed := bytes.NewBuffer(nil)
	if d.flowConfigDiscrete != nil {
		if err := binary.Write(typed, binary.NativeEndian, d.flowConfigDiscrete); err != nil {
			return err
		}
	} else if d.flowConfigContinuous != nil {
		if err := binary.Write(typed, binary.NativeEndian, d.flowConfigContinuous); err != nil {
			return err
		}
	}

	if typed.Len() > len(d.flowInfo.Config.Typed) {
		return fmt.Errorf("typed flow config is %d bytes, at most %d are allowed", typed.Len(), len(d.flowInfo.Config.Typed))
	}

	d.flowInfo.Config.Typed = [64]byte{}
	copy(d.flowInfo.Config.Typed[:], typed.Bytes())

	return nil
}

func (d *DummyFlow) writeFlowInfo() error {
	if err := d.encodeTypedConfig(); err != nil {
		return err
	}

	fd, err := os.OpenFile(filepath.Join(d.flowDir, "data"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o0644)
	if err != nil {
		return err
	}
	defer func() { _ = fd.Close() }()

	return binary.Write(fd, binary.NativeEndian, d.flowInfo)
}

func (d *DummyFlow) Create() error {
	if err := d.createTmpFlowDir(); err != nil {
		return err
	}
	if err := d.writeFlowDef(); err != nil {
		return err
	}
	if err := d.writeFlowInfo(); err != nil {
		return err
	}

	flowDir := filepath.Join(d.domain, d.flowDef.ID+".mxl-flow")
	if err := os.Rename(d.flowDir, flowDir); err != nil {
		return err
	}

	d.flowDir = flowDir
	return nil
}

// Updates the runtime info of a created flow in place, without replacing the
// data file, so that readers holding a mapping of it observe the change.
func (d *DummyFlow) UpdateRuntime(runtime mxlsys.FlowRuntimeInfo) error {
	if d.flowDir == "" {
		return os.ErrNotExist
	}

	d.flowInfo.Runtime = runtime

	fd, err := os.OpenFile(filepath.Join(d.flowDir, "data"), os.O_WRONLY, 0o0644)
	if err != nil {
		return err
	}
	defer func() { _ = fd.Close() }()

	return binary.Write(fd, binary.NativeEndian, d.flowInfo)
}

func (d *DummyFlow) Remove() error {
	return os.RemoveAll(d.flowDir)
}
