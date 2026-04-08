package crypto

// 5gaka.go — 5G-AKA key derivation on top of standard Milenage.
//
// References:
//   3GPP TS 33.501 Annex A.2 — KAUSF derivation
//   3GPP TS 33.501 Annex A.4 — XRES* derivation
//
// Milenage itself (RAND, XRES, CK, IK, AUTN) is computed by the existing
// GenerateEAPAKAVector function; we build KAUSF and XRES* on top of those outputs.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"

	"github.com/svinson1121/vectorcore-hss/internal/models"
	"github.com/svinson1121/vectorcore-hss/internal/repository"
)

// FiveGAKAVector holds the 5G-AKA authentication vector returned to the AUSF.
type FiveGAKAVector struct {
	RAND     []byte // 16 bytes — random challenge
	AUTN     []byte // 16 bytes — authentication token (same as 4G)
	XRESStar []byte // 16 bytes — expected response (5G variant)
	KAUSF    []byte // 32 bytes — anchor key for the AUSF
}

// Generate5GAKAVector produces a single 5G-AKA vector for the given subscriber.
// snn is the Serving Network Name, e.g. "5G:mnc435.mcc311.3gppnetwork.org".
// The SQN is atomically incremented in the database exactly once.
func Generate5GAKAVector(
	auc *models.AUC,
	profile *models.AlgorithmProfile,
	snn string,
	store repository.Repository,
	ctx context.Context,
) (*FiveGAKAVector, error) {
	// Step 1: Run standard Milenage to get CK, IK, XRES, AUTN, RAND.
	// GenerateEAPAKAVector handles SQN increment + Ki/OPc decoding.
	base, err := GenerateEAPAKAVector(auc, profile, store, ctx)
	if err != nil {
		return nil, err
	}

	key := append(base.CK, base.IK...) // 32-byte key: CK || IK
	snnBytes := []byte(snn)

	// Step 2: Derive KAUSF (TS 33.501 Annex A.2)
	//   FC    = 0x6A
	//   Input = FC || SNN || len(SNN,2B) || (SQN XOR AK) || 0x0006
	// SQN XOR AK is the first 6 bytes of AUTN (already computed by Milenage).
	sqnXorAK := base.AUTN[0:6]

	kausfInput := make([]byte, 0, 1+len(snnBytes)+2+6+2)
	kausfInput = append(kausfInput, 0x6A)
	kausfInput = append(kausfInput, snnBytes...)
	kausfInput = appendUint16(kausfInput, uint16(len(snnBytes)))
	kausfInput = append(kausfInput, sqnXorAK...)
	kausfInput = appendUint16(kausfInput, 0x0006)

	kausf := hmacSHA256(key, kausfInput) // 32 bytes

	// Step 3: Derive XRES* (TS 33.501 Annex A.4)
	//   FC    = 0x6B
	//   Input = FC || SNN || len(SNN,2B) || RAND || 0x0010 || XRES || len(XRES,2B)
	//   XRES* = HMAC-SHA-256(Key, Input)[16:32]  (last 16 bytes)
	xresStarInput := make([]byte, 0, 1+len(snnBytes)+2+16+2+len(base.XRES)+2)
	xresStarInput = append(xresStarInput, 0x6B)
	xresStarInput = append(xresStarInput, snnBytes...)
	xresStarInput = appendUint16(xresStarInput, uint16(len(snnBytes)))
	xresStarInput = append(xresStarInput, base.RAND...)
	xresStarInput = appendUint16(xresStarInput, uint16(len(base.RAND)))
	xresStarInput = append(xresStarInput, base.XRES...)
	xresStarInput = appendUint16(xresStarInput, uint16(len(base.XRES)))

	full := hmacSHA256(key, xresStarInput)
	xresStar := full[16:32] // last 16 bytes

	return &FiveGAKAVector{
		RAND:     base.RAND,
		AUTN:     base.AUTN,
		XRESStar: xresStar,
		KAUSF:    kausf,
	}, nil
}

// ResyncAnd5GAKAVector handles an AUTS resync and re-generates a 5G-AKA vector.
// randHex and autsHex are the hex strings received from the AUSF.
func ResyncAnd5GAKAVector(
	auc *models.AUC,
	profile *models.AlgorithmProfile,
	snn string,
	randBytes, autsBytes []byte,
	store repository.Repository,
	ctx context.Context,
) (*FiveGAKAVector, error) {
	// Reconstruct the 30-byte resyncInfo expected by ResyncSQNFull: RAND(16) || AUTS(14)
	resyncInfo := append(randBytes, autsBytes...)
	newSQN, err := ResyncSQNFull(auc, profile, resyncInfo)
	if err != nil {
		return nil, err
	}
	if err := store.ResyncSQN(ctx, auc.AUCID, newSQN); err != nil {
		return nil, err
	}
	// Reload AUC with updated SQN.
	updated, err := store.GetAUCByID(ctx, auc.AUCID)
	if err != nil {
		return nil, err
	}
	return Generate5GAKAVector(updated, profile, snn, store, ctx)
}

// hmacSHA256 computes HMAC-SHA-256(key, data).
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func appendUint16(b []byte, v uint16) []byte {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	return append(b, buf[:]...)
}
