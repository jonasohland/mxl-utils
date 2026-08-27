package mxl_test

import (
	"testing"

	"github.com/jonasohland/mxl-utils/pkg/mxl"
	"github.com/stretchr/testify/require"
)

const groupHintTag = "urn:x-nmos:tag:grouphint/v1.0"

func TestGetGroupHint(t *testing.T) {
	for _, tc := range []struct {
		name string
		tags map[string][]string
		hint mxl.GroupHint
		err  error
	}{
		{
			name: "name and role",
			tags: map[string][]string{groupHintTag: {"Camera 1:video"}},
			hint: mxl.GroupHint{Name: "Camera 1", Type: "video"},
		},
		{
			name: "name containing a colon",
			tags: map[string][]string{groupHintTag: {"Studio A:Camera 1:audio"}},
			hint: mxl.GroupHint{Name: "Studio A:Camera 1", Type: "audio"},
		},
		{
			name: "empty role",
			tags: map[string][]string{groupHintTag: {"Camera 1:"}},
			hint: mxl.GroupHint{Name: "Camera 1", Type: ""},
		},
		{
			name: "empty name",
			tags: map[string][]string{groupHintTag: {":video"}},
			hint: mxl.GroupHint{Name: "", Type: "video"},
		},
		{
			name: "no tags at all",
			tags: nil,
			err:  mxl.ErrMissingGroupHint,
		},
		{
			name: "unrelated tags only",
			tags: map[string][]string{"urn:x-nmos:tag:other/v1.0": {"whatever"}},
			err:  mxl.ErrMissingGroupHint,
		},
		{
			name: "empty tag value",
			tags: map[string][]string{groupHintTag: {}},
			err:  mxl.ErrInvalidGroupHint,
		},
		{
			name: "more than one tag value",
			tags: map[string][]string{groupHintTag: {"Camera 1:video", "Camera 2:video"}},
			err:  mxl.ErrInvalidGroupHint,
		},
		{
			name: "no separator",
			tags: map[string][]string{groupHintTag: {"Camera 1"}},
			err:  mxl.ErrInvalidGroupHint,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hint, err := (&mxl.FlowDefinition{Tags: tc.tags}).GetGroupHint()

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.Nil(t, hint)
				return
			}

			require.NoError(t, err)
			require.Equal(t, &tc.hint, hint)
		})
	}
}
