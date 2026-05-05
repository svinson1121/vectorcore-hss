package gx

import (
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

func TestApplyTFTHandlingStandardLeavesPermitInUnchanged(t *testing.T) {
	in := "permit in 17 from 1.1.1.1 50021 to 2.2.2.2 54939"
	got, rewritten := ApplyTFTHandling(in, "standard")
	if rewritten {
		t.Fatal("expected no rewrite")
	}
	if got != in {
		t.Fatalf("expected original TFT, got %q", got)
	}
}

func TestApplyTFTHandlingStandardLeavesPermitOutUnchanged(t *testing.T) {
	in := "permit out 17 from 2.2.2.2 54939 to 1.1.1.1 50021"
	got, rewritten := ApplyTFTHandling(in, "standard")
	if rewritten {
		t.Fatal("expected no rewrite")
	}
	if got != in {
		t.Fatalf("expected original TFT, got %q", got)
	}
}

func TestApplyTFTHandlingFlipPermitInRewrites(t *testing.T) {
	got, rewritten := ApplyTFTHandling("permit in 17 from 1.1.1.1 50021 to 2.2.2.2 54939", "flip-permit-in")
	if !rewritten {
		t.Fatal("expected rewrite")
	}
	want := "permit out 17 from 2.2.2.2 54939 to 1.1.1.1 50021"
	if got != want {
		t.Fatalf("unexpected TFT rewrite: got %q want %q", got, want)
	}
}

func TestApplyTFTHandlingFlipPermitInLeavesPermitOutUnchanged(t *testing.T) {
	in := "permit out 17 from 2.2.2.2 54939 to 1.1.1.1 50021"
	got, rewritten := ApplyTFTHandling(in, "flip-permit-in")
	if rewritten {
		t.Fatal("expected no rewrite")
	}
	if got != in {
		t.Fatalf("expected original TFT, got %q", got)
	}
}

func TestApplyTFTHandlingFlipPermitInIgnoresMalformed(t *testing.T) {
	in := "permit in 17 from 1.1.1.1 to 2.2.2.2"
	got, rewritten := ApplyTFTHandling(in, "flip-permit-in")
	if rewritten {
		t.Fatal("expected malformed TFT to remain unchanged")
	}
	if got != in {
		t.Fatalf("expected original TFT, got %q", got)
	}
}

func TestBuildChargingRuleDefinitionProcessesMultipleTFTsIndependently(t *testing.T) {
	rule := models.ChargingRule{RuleName: "rule1", MBRDown: 1, MBRUp: 1}
	tfts := []models.TFT{
		{TFTString: "permit in 17 from 1.1.1.1 50021 to 2.2.2.2 54939", Direction: 1},
		{TFTString: "permit out ip from any to any", Direction: 2},
	}

	avp := buildChargingRuleDefinition(rule, tfts, "", "flip-permit-in", zap.NewNop())
	got := flowDescriptions(t, avp)
	want := []string{
		"permit out 17 from 2.2.2.2 54939 to 1.1.1.1 50021",
		"permit out ip from any to any",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d flow descriptions, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("flow %d mismatch: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestBuildChargingRuleDefinitionLogsMalformedPermitInWarning(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	rule := models.ChargingRule{RuleName: "rule1", MBRDown: 1, MBRUp: 1}
	tfts := []models.TFT{{TFTString: "permit in 17 from 1.1.1.1 to 2.2.2.2", Direction: 1}}

	_ = buildChargingRuleDefinition(rule, tfts, "", "flip-permit-in", logger)

	entries := logs.FilterMessage(`Unable to rewrite malformed permit-in TFT; passing unchanged: "permit in 17 from 1.1.1.1 to 2.2.2.2"`).All()
	if len(entries) != 1 {
		t.Fatalf("expected malformed TFT warning, got %d matching entries", len(entries))
	}
}

func flowDescriptions(t *testing.T, ruleAVP *diam.AVP) []string {
	t.Helper()
	ruleGroup, ok := ruleAVP.Data.(*diam.GroupedAVP)
	if !ok {
		t.Fatalf("expected charging rule grouped AVP, got %T", ruleAVP.Data)
	}

	var out []string
	for _, child := range ruleGroup.AVP {
		if child.Code != avpFlowInformation {
			continue
		}
		flowGroup, ok := child.Data.(*diam.GroupedAVP)
		if !ok {
			t.Fatalf("expected flow information grouped AVP, got %T", child.Data)
		}
		for _, flowChild := range flowGroup.AVP {
			if flowChild.Code != avpFlowDescription {
				continue
			}
			desc, ok := flowChild.Data.(datatype.IPFilterRule)
			if !ok {
				t.Fatalf("expected IPFilterRule, got %T", flowChild.Data)
			}
			out = append(out, string(desc))
		}
	}
	return out
}
