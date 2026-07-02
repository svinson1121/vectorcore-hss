package rx

import (
	"strings"
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/config"
)

func TestApplyTFTHandlingUsesPCRFModeForRelayedIMSFlows(t *testing.T) {
	h := NewHandlers(&config.Config{
		PCRF: config.PCRFConfig{TFTHandling: "flip-permit-in"},
	}, nil, zap.NewNop(), nil)

	got, rewritten := h.applyTFTHandling("permit in 17 from 1.1.1.1 to 2.2.2.2")
	if !rewritten {
		t.Fatal("expected IMS flow to be rewritten")
	}

	want := "permit out 17 from 2.2.2.2 to 1.1.1.1"
	if got != want {
		t.Fatalf("unexpected TFT rewrite: got %q want %q", got, want)
	}
}

func TestBuildRxFlowInformationPreservesOriginalDirectionWithFlipPermitIn(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		fd         string
		wantPrefix string
		wantDir    uint32
	}{
		{
			name:       "downlink permit out",
			mode:       "flip-permit-in",
			fd:         "permit out 17 from 10.46.0.64 49120 to 10.46.0.61 1240",
			wantPrefix: "permit out",
			wantDir:    flowDirectionDownlink,
		},
		{
			name:       "uplink permit in rewritten to permit out",
			mode:       "flip-permit-in",
			fd:         "permit in 17 from 10.46.0.61 1240 to 10.46.0.64 49120",
			wantPrefix: "permit out",
			wantDir:    flowDirectionUplink,
		},
		{
			name:       "uplink permit in unchanged in normal mode",
			fd:         "permit in 17 from 10.46.0.61 1240 to 10.46.0.64 49120",
			wantPrefix: "permit in",
			wantDir:    flowDirectionUplink,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandlers(&config.Config{
				PCRF: config.PCRFConfig{TFTHandling: tt.mode},
			}, nil, zap.NewNop(), nil)

			desc, dir := rxFlowInformationValues(t, h.buildRxFlowInformation(tt.fd))

			if !strings.HasPrefix(desc, tt.wantPrefix) {
				t.Fatalf("unexpected Flow-Description: got %q want prefix %q", desc, tt.wantPrefix)
			}
			if dir != tt.wantDir {
				t.Fatalf("unexpected Flow-Direction: got %d want %d", dir, tt.wantDir)
			}
		})
	}
}

func TestDetectFlowDirectionFromRxFlowDescription(t *testing.T) {
	tests := []struct {
		fd      string
		wantDir uint32
	}{
		{fd: "permit out ip from any to any", wantDir: flowDirectionDownlink},
		{fd: " permit in ip from any to any", wantDir: flowDirectionUplink},
		{fd: "permit ip from any to any", wantDir: flowDirectionBidirectional},
	}

	for _, tt := range tests {
		if got := detectFlowDirectionFromRxFlowDescription(tt.fd); got != tt.wantDir {
			t.Fatalf("detectFlowDirectionFromRxFlowDescription(%q) = %d, want %d", tt.fd, got, tt.wantDir)
		}
	}
}

func rxFlowInformationValues(t *testing.T, flowAVP *diam.AVP) (string, uint32) {
	t.Helper()

	flowGroup, ok := flowAVP.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("expected flow information grouped AVP, got %T", flowAVP.Data)
	}

	var desc string
	var dir uint32
	for _, child := range flowGroup.AVP {
		switch child.Code {
		case avpFlowDescription:
			value, ok := child.Data.(datatype.IPFilterRule)
			if !ok {
				t.Fatalf("expected IPFilterRule, got %T", child.Data)
			}
			desc = string(value)
		case avpGxFlowDirection:
			value, ok := child.Data.(datatype.Enumerated)
			if !ok {
				t.Fatalf("expected Enumerated, got %T", child.Data)
			}
			dir = uint32(value)
		}
	}

	if desc == "" {
		t.Fatal("missing Flow-Description")
	}
	if dir == 0 {
		t.Fatal("missing Flow-Direction")
	}

	return desc, dir
}
