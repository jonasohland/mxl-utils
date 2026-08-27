package mxlsys

type Rational struct {
	Numerator   int64
	Denominator int64
}

type CommonFlowConfigInfo struct {
	ID                     [16]byte
	Format                 uint32
	Flags                  uint32
	Rate                   Rational
	MaxCommitBatchSizeHint uint32
	MaxSyncBatchSizeHint   uint32
	PayloadLocation        uint32
	DeviceIndex            uint32
	Reserved               [72]byte
}

type ContinuousFlowConfigInfo struct {
	Channels     uint32
	BufferLength uint32
	Reserved     [56]byte
}

type DiscreteFlowConfigInfo struct {
	SliceSizes [4]uint32
	GrainCount uint32
	Reserved   [44]byte
}

type FlowConfigInfo struct {
	Common CommonFlowConfigInfo
	Typed  [64]byte
}

type FlowRuntimeInfo struct {
	HeadIndex     uint64
	LastWriteTime uint64
	LastReadTime  uint64
	Reserved      [40]byte
}

type FlowInfo struct {
	Version  uint32
	Size     uint32
	Config   FlowConfigInfo
	Runtime  FlowRuntimeInfo
	Reserved [1784]byte
}
