package s6a

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// checkRoaming evaluates whether a subscriber is permitted to attach via the
// visited PLMN encoded in visitedPLMN (3-byte BCD, same format as ULI PLMN).
//
// Decision order:
//  1. Home network → always allow.
//  2. sub.RoamingEnabled == false → deny.
//  3. No enabled rule in roaming_rules for this MCC/MNC → apply allow_undefined_networks.
//  4. Rule found → apply its Allow flag.
func (h *Handlers) checkRoaming(ctx context.Context, sub *models.Subscriber, visitedPLMN []byte) error {
	if len(visitedPLMN) != 3 {
		return nil // can't determine visited network — fail open
	}

	visitMCC, visitMNC := decodePLMN(visitedPLMN)

	// Home network — not roaming.
	if visitMCC == h.homeMCC && visitMNC == h.homeMNC {
		return nil
	}

	h.log.Debug("s6a: roaming check",
		zap.String("imsi", sub.IMSI),
		zap.String("visit_mcc", visitMCC),
		zap.String("visit_mnc", visitMNC),
	)

	// Subscriber master roaming switch.
	if sub.RoamingEnabled != nil && !*sub.RoamingEnabled {
		return fmt.Errorf("roaming disabled for subscriber")
	}

	// Look up the visiting network in roaming_rules.
	rule, err := h.store.GetRoamingRuleByMCCMNC(ctx, visitMCC, visitMNC)
	if err == repository.ErrNotFound {
		if !h.allowUndefinedRoaming {
			return fmt.Errorf("roaming not allowed: network %s/%s not defined", visitMCC, visitMNC)
		}
		return nil
	}
	if err != nil {
		// DB error — fail open to avoid blocking legitimate attaches.
		h.log.Warn("s6a: roaming rule lookup failed, failing open", zap.Error(err))
		return nil
	}

	if rule.Allow != nil && !*rule.Allow {
		return fmt.Errorf("roaming denied for network %s/%s", visitMCC, visitMNC)
	}
	return nil
}
