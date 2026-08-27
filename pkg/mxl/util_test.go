package mxl_test

import (
	"testing"

	"github.com/jonasohland/mxl-utils/pkg/mxl"
	"github.com/stretchr/testify/require"
)

const testFlowID = "ae8b77bd-a796-4aed-90ad-57a7dc388f85"

func TestGetFlowIDFromPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		id   string
		ok   bool
	}{
		{name: "bare directory name", path: testFlowID + ".mxl-flow", id: testFlowID, ok: true},
		{name: "absolute path", path: "/mnt/domain/" + testFlowID + ".mxl-flow", id: testFlowID, ok: true},
		{name: "relative path", path: "domain/" + testFlowID + ".mxl-flow", id: testFlowID, ok: true},
		{name: "trailing separator", path: "/mnt/domain/" + testFlowID + ".mxl-flow/", id: testFlowID, ok: true},
		{name: "uppercase id", path: "AE8B77BD-A796-4AED-90AD-57A7DC388F85.mxl-flow", id: "AE8B77BD-A796-4AED-90AD-57A7DC388F85", ok: true},
		{name: "empty", path: ""},
		{name: "missing suffix", path: testFlowID},
		{name: "wrong suffix", path: testFlowID + ".mxl-grain"},
		{name: "suffix not at the end", path: testFlowID + ".mxl-flow.bak"},
		{name: "no id", path: ".mxl-flow"},
		{name: "id too short", path: "ae8b77bd.mxl-flow"},
		{name: "id without dashes", path: "ae8b77bda7964aed90ad57a7dc388f85.mxl-flow"},
		{name: "urn prefixed id", path: "urn:uuid:" + testFlowID + ".mxl-flow"},
		{name: "not a uuid", path: "zzzzzzzz-a796-4aed-90ad-57a7dc388f85.mxl-flow"},
		{name: "grain directory", path: "/mnt/domain/" + testFlowID + ".mxl-flow/grains"},
		{name: "data file", path: "/mnt/domain/" + testFlowID + ".mxl-flow/data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := mxl.GetFlowIDFromPath(tc.path)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.id, id)

			require.Equal(t, tc.ok, mxl.IsFlowDir(tc.path))
		})
	}
}
