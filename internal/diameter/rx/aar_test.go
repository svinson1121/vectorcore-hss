package rx

import (
	"testing"

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
