package testutil

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jonasohland/mxl-utils/pkg/mxl"
)

type FlowSize = int
type FlowRate = int

const (
	FlowSize1080 FlowSize = 0
	FlowSize2160 FlowSize = 1
)

const (
	FlowRate25 FlowRate = 0
	FlowRate50 FlowRate = 1
	FlowRate23 FlowRate = 2
	FlowRate59 FlowSize = 3
)

func makeGroupHintTag(cookie, ty string) map[string][]string {
	return map[string][]string{
		"urn:x-nmos:tag:grouphint/v1.0": {fmt.Sprintf("Test Flow %s:%s", cookie, ty)},
	}
}

func initFlowDefRandom() (string, *mxl.FlowDefinition) {
	cookie := generateRandomString(16)
	return cookie, &mxl.FlowDefinition{
		ID:          uuid.New().String(),
		Label:       fmt.Sprintf("Test Flow %s", cookie),
		Description: fmt.Sprintf("Randomly generated test flow %s", cookie),
	}
}

func NewVideoFlowDef(size FlowSize, rate FlowRate) *mxl.FlowDefinition {
	cookie, fd := initFlowDefRandom()

	fd.MediaType = "video/v210"
	fd.Tags = makeGroupHintTag(cookie, "video")
	fd.Format = "urn:x-nmos:format:video"
	fd.InterlaceMode = "progressive"

	switch size {
	case FlowSize1080:
		fd.FrameWidth = new(1920)
		fd.FrameHeight = new(1080)
	case FlowSize2160:
		fd.FrameWidth = new(3840)
		fd.FrameHeight = new(2160)
	}

	switch rate {
	case FlowRate25:
		fd.GrainRate = &mxl.Rational{
			Numerator:   25,
			Denominator: 1,
		}
	case FlowRate50:
		fd.GrainRate = &mxl.Rational{
			Numerator:   50,
			Denominator: 1,
		}
	case FlowRate23:
		fd.GrainRate = &mxl.Rational{
			Numerator:   24000,
			Denominator: 1001,
		}
	case FlowRate59:
		fd.GrainRate = &mxl.Rational{
			Numerator:   60000,
			Denominator: 1001,
		}
	}

	return fd
}

func NewAudioFlowDef() *mxl.FlowDefinition {
	cookie, fd := initFlowDefRandom()
	fd.MediaType = "audio/float32"
	fd.Format = "urn:x-nmos:format:audio"
	fd.Tags = makeGroupHintTag(cookie, "audio")
	fd.SampleRate = &mxl.Rational{Numerator: 48000, Denominator: 1}
	fd.ChannelCount = new(64)

	return fd
}

func RandomVideoFlow(domain string) (*DummyFlow, error) {
	return NewDummyFlow(domain, NewVideoFlowDef(FlowSize1080, FlowRate50))
}

func RandomAudioFlow(domain string) (*DummyFlow, error) {
	return NewDummyFlow(domain, NewAudioFlowDef())
}
