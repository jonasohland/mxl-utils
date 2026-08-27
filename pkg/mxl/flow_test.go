package mxl_test

import (
	"testing"

	"github.com/jonasohland/mxl-utils/pkg/mxl"
	"github.com/jonasohland/mxl-utils/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func TestOpenFlow(t *testing.T) {
	flow, err := testutil.NewDummyFlow("test", &mxl.FlowDefinition{ID: "ae8b77bd-a796-4aed-90ad-57a7dc388f85", MediaType: "video/v210", GrainRate: &mxl.Rational{Numerator: 25, Denominator: 1}})
	require.NoError(t, err)
	require.NoError(t, flow.Create())
	require.NoError(t, flow.Remove())
}
