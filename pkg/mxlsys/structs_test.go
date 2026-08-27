package mxlsys_test

import (
	"encoding/binary"
	"testing"

	"github.com/jonasohland/mxl-utils/pkg/mxlsys"
	"github.com/stretchr/testify/require"
)

// The structures in this package mirror the binary layout of the flow info as
// defined in <mxl/flowinfo.h>. The header pads every structure to a fixed size,
// so a mismatch here means the mapping is decoded at the wrong offsets.
func TestStructSizes(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
		of   any
	}{
		{name: "mxlRational", size: 16, of: mxlsys.Rational{}},
		{name: "mxlCommonFlowConfigInfo", size: 128, of: mxlsys.CommonFlowConfigInfo{}},
		{name: "mxlContinuousFlowConfigInfo", size: 64, of: mxlsys.ContinuousFlowConfigInfo{}},
		{name: "mxlDiscreteFlowConfigInfo", size: 64, of: mxlsys.DiscreteFlowConfigInfo{}},
		{name: "mxlFlowConfigInfo", size: 192, of: mxlsys.FlowConfigInfo{}},
		{name: "mxlFlowRuntimeInfo", size: 64, of: mxlsys.FlowRuntimeInfo{}},
		{name: "mxlFlowInfo", size: 2048, of: mxlsys.FlowInfo{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.size, binary.Size(tc.of))
		})
	}
}

// The device index is signed, it is -1 for flows whose payload lives in host
// memory.
func TestDeviceIndexIsSigned(t *testing.T) {
	info := mxlsys.CommonFlowConfigInfo{DeviceIndex: -1}
	require.Equal(t, int32(-1), info.DeviceIndex)
}
