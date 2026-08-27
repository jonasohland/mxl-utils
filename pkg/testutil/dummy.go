package testutil

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jonasohland/mxl-utils/pkg/mxl"
	"github.com/jonasohland/mxl-utils/pkg/mxlsys"
	"github.com/samber/lo"
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

	flow := &DummyFlow{
		flowDef: flowDef,
		flowInfo: &mxlsys.FlowInfo{
			Version: 1,
			Size:    2048,
			Config: mxlsys.FlowConfigInfo{
				Common: mxlsys.CommonFlowConfigInfo{
					ID:     id,
					Format: 0,
					Flags:  0,
					Rate: mxlsys.Rational{
						Numerator:   flowDef.GrainRate.Numerator,
						Denominator: flowDef.GrainRate.Denominator,
					},
					MaxCommitBatchSizeHint: 0,
					MaxSyncBatchSizeHint:   0,
					PayloadLocation:        0,
					DeviceIndex:            0,
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

	switch flowDef.MediaType {
	case "audio/float32":
		flow.flowConfigContinuous = &mxlsys.ContinuousFlowConfigInfo{
			Channels:     lo.Ternary(flowDef.ChannelCount == nil, 0, uint32(*flowDef.ChannelCount)),
			BufferLength: 0,
		}
	case "video/v210", "video/smpte291":
		flow.flowConfigDiscrete = &mxlsys.DiscreteFlowConfigInfo{
			SliceSizes: [4]uint32{1, 0, 0, 0},
			GrainCount: 11,
		}
	default:
		return nil, errors.New("invalid flow media type")
	}

	return flow, nil
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

func (d *DummyFlow) writeFlowInfo() error {
	typed := bytes.NewBuffer(nil)
	if d.flowConfigDiscrete != nil {
		if err := binary.Write(typed, binary.NativeEndian, d.flowConfigDiscrete); err != nil {
			return err
		}
	} else if d.flowConfigContinuous != nil {
		if err := binary.Write(typed, binary.NativeEndian, d.flowConfigDiscrete); err != nil {
			return err
		}
	}

	d.flowInfo.Config.Typed = [64]byte(typed.Bytes())

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

func (d *DummyFlow) Remove() error {
	return os.RemoveAll(d.flowDir)
}
