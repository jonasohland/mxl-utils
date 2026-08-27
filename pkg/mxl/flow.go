package mxl

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	mmap "github.com/edsrzf/mmap-go"
	"github.com/google/uuid"
	"github.com/jonasohland/mxl-utils/pkg/mxlsys"
)

var (
	ErrMissingGroupHint = errors.New("missing group hint")
	ErrInvalidGroupHint = errors.New("invalid group hint")
)

type DataFormat string
type PayloadLocation string

const (
	DataFormatUnspec DataFormat = "unspec"
	DataFormatVideo  DataFormat = "video"
	DataFormatAudio  DataFormat = "audio"
	DataFormatData   DataFormat = "data"
)

const (
	PayloadLocationHost   PayloadLocation = "host"
	PayloadLocationDevice PayloadLocation = "device"
)

var dataFormatMap = map[uint32]DataFormat{
	0: DataFormatUnspec,
	1: DataFormatVideo,
	2: DataFormatAudio,
	3: DataFormatData,
}

var payloadLocationMap = map[uint32]PayloadLocation{
	0: PayloadLocationHost,
	1: PayloadLocationDevice,
}

type Rational struct {
	Numerator   int64
	Denominator int64
}

type DiscreteFlowConfigInfo struct {
	SliceSizes [4]uint32
	GrainCount uint32
}

type ContinuousFlowConfigInfo struct {
	Channels     uint32
	BufferLength uint32
}

type FlowConfigInfo struct {
	ID                     string
	Format                 DataFormat
	Rate                   Rational
	MaxCommitBatchSizeHint int
	MaxSyncBatchSizeHint   int
	PayloadLocation        PayloadLocation
	DeviceIndex            int

	Discrete   *DiscreteFlowConfigInfo
	Continuous *ContinuousFlowConfigInfo
}

type FlowRuntimeInfo struct {
	HeadIndex     uint64
	LastWriteTime uint64
	LastReadTime  uint64
}

type FlowTags struct {
	GroupHint []string `json:"urn:x-nmos:tag:grouphint/v1.0"`
}

type Component struct {
	Name     string `json:"name"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	BitDepth int    `json:"bit_depth"`
}

type GroupHint struct {
	Name string
	Type string
}

type FlowDefinition struct {
	Copyright     string              `json:"$copyright"` // probably not meant to be actually read by any applications
	License       string              `json:"$license"`   // probably not meant to be actually read by any applications
	Description   string              `json:"description"`
	SourceID      string              `json:"source_id,omitempty"`
	DeviceID      string              `json:"device_id,omitempty"`
	ID            string              `json:"id"`
	Tags          map[string][]string `json:"tags"`
	Format        string              `json:"format"`
	Label         string              `json:"label"`
	Version       string              `json:"version"`
	Parents       []string            `json:"parents"`
	MediaType     string              `json:"media_type"`
	GrainRate     *Rational           `json:"grain_rate,omitempty"`
	SampleRate    *Rational           `json:"sample_rate,omitempty"`
	FrameWidth    *int                `json:"frame_width,omitempty"`
	FrameHeight   *int                `json:"frame_height,omitempty"`
	InterlaceMode string              `json:"interlace_mode,omitempty"`
	Colorspace    string              `json:"colorspace,omitempty"`
	Componenets   []Component         `json:"components,omitempty"`
	ChannelCount  *int                `json:"channel_count,omitempty"`
	BitDepth      *int                `json:"bit_depth,omitempty"`
}

func (d *FlowDefinition) GetGroupHint() (*GroupHint, error) {
	gh, ok := d.Tags["urn:x-nmos:tag:grouphint/v1.0"]
	if !ok {
		return nil, ErrMissingGroupHint
	}

	if len(gh) != 1 {
		return nil, ErrInvalidGroupHint
	}

	parts := strings.Split(gh[0], ":")
	if len(parts) < 2 {
		return nil, ErrInvalidGroupHint
	}

	return &GroupHint{
		Name: strings.Join(parts[:len(parts)-1], ":"),
		Type: parts[len(parts)-1],
	}, nil
}

type Flow struct {
	dir string
	fd  *os.File
	mm  mmap.MMap
}

func Open(domain string, id string) (*Flow, error) {
	dir := filepath.Join(domain, id+".mxl-flow")
	filename := filepath.Join(dir, "data")
	fd, err := os.OpenFile(filename, os.O_RDONLY, 0000)
	if err != nil {
		return nil, err
	}

	mm, err := mmap.Map(fd, mmap.RDONLY, 0)
	if err != nil {
		_ = fd.Close()
		return nil, err
	}

	return &Flow{dir: dir, fd: fd, mm: mm}, nil
}

func (f *Flow) Close() error {
	return errors.Join(f.mm.Unmap(), f.fd.Close())
}

func (f *Flow) IsValid() bool {
	myIno, err := fgetIno(f.fd)
	if err != nil {
		return false
	}

	fsIno, err := getIno(filepath.Join(f.dir, "data"))
	if err != nil {
		return false
	}

	return myIno == fsIno
}

func (f *Flow) GetInfo() (*FlowConfigInfo, *FlowRuntimeInfo, error) {
	var raw mxlsys.FlowInfo
	if err := binary.Read(bytes.NewReader(f.mm), binary.NativeEndian, &raw); err != nil {
		return nil, nil, err
	}

	id, err := uuid.FromBytes(raw.Config.Common.ID[:])
	if err != nil {
		return nil, nil, err
	}

	configInfo := &FlowConfigInfo{
		ID:     id.String(),
		Format: dataFormatMap[raw.Config.Common.Format],
		Rate: Rational{
			Numerator:   raw.Config.Common.Rate.Numerator,
			Denominator: raw.Config.Common.Rate.Denominator,
		},
		MaxCommitBatchSizeHint: int(raw.Config.Common.MaxCommitBatchSizeHint),
		MaxSyncBatchSizeHint:   int(raw.Config.Common.MaxSyncBatchSizeHint),
		PayloadLocation:        payloadLocationMap[raw.Config.Common.PayloadLocation],
		DeviceIndex:            int(raw.Config.Common.DeviceIndex),
	}

	if configInfo.Format == DataFormatData || configInfo.Format == DataFormatVideo {
		var craw mxlsys.DiscreteFlowConfigInfo
		if err := binary.Read(bytes.NewReader(raw.Config.Typed[:]), binary.NativeEndian, &craw); err != nil {
			return nil, nil, err
		}

		configInfo.Discrete = &DiscreteFlowConfigInfo{
			SliceSizes: craw.SliceSizes,
			GrainCount: craw.GrainCount,
		}
	} else {
		var craw mxlsys.ContinuousFlowConfigInfo
		if err := binary.Read(bytes.NewReader(raw.Config.Typed[:]), binary.NativeEndian, &craw); err != nil {
			return nil, nil, err
		}

		configInfo.Continuous = &ContinuousFlowConfigInfo{
			Channels:     craw.Channels,
			BufferLength: craw.BufferLength,
		}
	}

	return configInfo,
		&FlowRuntimeInfo{
			HeadIndex:     raw.Runtime.HeadIndex,
			LastWriteTime: raw.Runtime.LastWriteTime,
			LastReadTime:  raw.Runtime.LastReadTime,
		},
		nil
}

func (f *Flow) GetDefinition() (*FlowDefinition, error) {
	fd, err := os.OpenFile(filepath.Join(f.dir, "flow_def.json"), os.O_RDONLY, 0000)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fd.Close() }()

	flowDef := &FlowDefinition{}
	if err := json.NewDecoder(fd).Decode(flowDef); err != nil {
		return nil, err
	}

	return flowDef, nil
}
