package crypto

// Custom Milenage implementation for non-standard algorithm profiles.
//
// 3GPP TS 35.206 defines the Milenage algorithm family (f1, f1*, f2–f5, f5*).
// The standard uses fixed c1–c5 and r1–r5 constants.  This file implements
// the same algorithm parametrically so subscribers can use custom constants
// stored in an AlgorithmProfile record.
//
// All rotation values (r1–r5) must be byte-aligned (multiples of 8 bits).
//
// Reference formulas (from 3GPP TS 35.206 §4.1):
//
//   TEMP = E_K(RAND XOR OPC)
//
//   f1/f1*:
//     IN1 = SQN || AMF || SQN || AMF   (128 bits)
//     OUT1 = E_K(TEMP XOR rotate(IN1 XOR OPC, r1) XOR c1) XOR OPC
//     MAC-A = OUT1[0:8],  MAC-S = OUT1[8:16]
//
//   f2/f5 (share one AES call):
//     OUT2 = E_K(rotate(TEMP XOR OPC, r2) XOR c2) XOR OPC
//     XRES = OUT2[8:16],  AK = OUT2[0:6]
//
//   f3:
//     OUT3 = E_K(rotate(TEMP XOR OPC, r3) XOR c3) XOR OPC  → CK
//
//   f4:
//     OUT4 = E_K(rotate(TEMP XOR OPC, r4) XOR c4) XOR OPC  → IK
//
//   f5*:
//     OUT5* = E_K(rotate(TEMP XOR OPC, r5) XOR c5) XOR OPC
//     AK* = OUT5*[0:6]
//
// Verified against 3GPP TS 35.207 Test Set 1.

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/svinson1121/vectorcore-hss/internal/models"
)

// profileConstants holds the decoded binary Milenage c/r constants.
type profileConstants struct {
	c1, c2, c3, c4, c5 [16]byte
	r1, r2, r3, r4, r5 int // rotation in bytes (bits / 8)
}

// decodeProfile parses an AlgorithmProfile into binary constants.
func decodeProfile(p *models.AlgorithmProfile) (*profileConstants, error) {
	decode := func(s, name string) ([16]byte, error) {
		b, err := hex.DecodeString(s)
		if err != nil || len(b) != 16 {
			return [16]byte{}, fmt.Errorf("algorithm profile: %s must be 32 hex chars: %w", name, err)
		}
		var out [16]byte
		copy(out[:], b)
		return out, nil
	}
	var pc profileConstants
	var err error
	if pc.c1, err = decode(p.C1, "c1"); err != nil {
		return nil, err
	}
	if pc.c2, err = decode(p.C2, "c2"); err != nil {
		return nil, err
	}
	if pc.c3, err = decode(p.C3, "c3"); err != nil {
		return nil, err
	}
	if pc.c4, err = decode(p.C4, "c4"); err != nil {
		return nil, err
	}
	if pc.c5, err = decode(p.C5, "c5"); err != nil {
		return nil, err
	}
	for _, rv := range []struct {
		v    int
		name string
	}{{p.R1, "r1"}, {p.R2, "r2"}, {p.R3, "r3"}, {p.R4, "r4"}, {p.R5, "r5"}} {
		if rv.v%8 != 0 {
			return nil, fmt.Errorf("algorithm profile: %s=%d is not byte-aligned (must be multiple of 8)", rv.name, rv.v)
		}
	}
	pc.r1 = (p.R1 / 8) % 16
	pc.r2 = (p.R2 / 8) % 16
	pc.r3 = (p.R3 / 8) % 16
	pc.r4 = (p.R4 / 8) % 16
	pc.r5 = (p.R5 / 8) % 16
	return &pc, nil
}

// xor16 XORs two 16-byte arrays.
func xor16(a, b [16]byte) [16]byte {
	var out [16]byte
	for i := range out {
		out[i] = a[i] ^ b[i]
	}
	return out
}

// rotateLeft128 rotates a 16-byte block left by n bytes: out[i] = in[(i+n) % 16].
func rotateLeft128(b [16]byte, n int) [16]byte {
	if n == 0 {
		return b
	}
	var out [16]byte
	for i := 0; i < 16; i++ {
		out[i] = b[(i+n)%16]
	}
	return out
}

// aesEncrypt performs a single AES-128 block encryption.
func aesEncrypt(key, pt [16]byte) ([16]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return [16]byte{}, err
	}
	var ct [16]byte
	block.Encrypt(ct[:], pt[:])
	return ct, nil
}

// milenageFN computes the generic Milenage f2/f3/f4/f5/f5* sub-function:
//
//	OUT = E_K(rotate(TEMP XOR OPC, r) XOR c) XOR OPC
func milenageFN(ki, opc, temp, c [16]byte, r int) ([16]byte, error) {
	in := xor16(rotateLeft128(xor16(temp, opc), r), c)
	out, err := aesEncrypt(ki, in)
	if err != nil {
		return [16]byte{}, err
	}
	return xor16(out, opc), nil
}

// milenageF1 computes f1 (MAC-A) and f1* (MAC-S):
//
//	IN1 = SQN[0..47] || AMF[0..15] || SQN[0..47] || AMF[0..15]
//	OUT1 = E_K(TEMP XOR rotate(IN1 XOR OPC, r1) XOR c1) XOR OPC
//	MAC-A = OUT1[0:8], MAC-S = OUT1[8:16]
func milenageF1(ki, opc, temp [16]byte, sqn uint64, amf [2]byte, c1 [16]byte, r1 int) (macA, macS [8]byte, err error) {
	sqnV := sqn // copy to variable so byte() truncates correctly at runtime
	sqnB := [6]byte{
		byte(sqnV >> 40), byte(sqnV >> 32), byte(sqnV >> 24),
		byte(sqnV >> 16), byte(sqnV >> 8), byte(sqnV),
	}
	var in1 [16]byte
	copy(in1[0:6], sqnB[:])
	copy(in1[6:8], amf[:])
	copy(in1[8:14], sqnB[:])
	copy(in1[14:16], amf[:])

	// TEMP XOR rotate(IN1 XOR OPC, r1) XOR c1
	in := xor16(xor16(temp, rotateLeft128(xor16(in1, opc), r1)), c1)
	out, e := aesEncrypt(ki, in)
	if e != nil {
		err = e
		return
	}
	out = xor16(out, opc)
	copy(macA[:], out[0:8])
	copy(macS[:], out[8:16])
	return
}

// milenageCore computes TEMP and derives all outputs for vector generation:
//
//	TEMP = E_K(RAND XOR OPC)
//	xres (f2)  = OUT2[8:16]
//	ak   (f5)  = OUT2[0:6]
//	ck   (f3)  = OUT3[0:16]
//	ik   (f4)  = OUT4[0:16]
//	macA (f1)  = OUT1[0:8]
func milenageCore(ki, opc, randB [16]byte, sqn uint64, amf [2]byte, pc *profileConstants) (
	xres [8]byte, ck, ik [16]byte, ak [6]byte, macA [8]byte, err error,
) {
	temp, err := aesEncrypt(ki, xor16(randB, opc))
	if err != nil {
		return
	}

	// f2/f5 (one AES call): XRES = out[8:16], AK = out[0:6]
	out25, e := milenageFN(ki, opc, temp, pc.c2, pc.r2)
	if e != nil {
		err = e
		return
	}
	copy(xres[:], out25[8:16])
	copy(ak[:], out25[0:6])

	// f3 → CK
	out3, e := milenageFN(ki, opc, temp, pc.c3, pc.r3)
	if e != nil {
		err = e
		return
	}
	ck = out3

	// f4 → IK
	out4, e := milenageFN(ki, opc, temp, pc.c4, pc.r4)
	if e != nil {
		err = e
		return
	}
	ik = out4

	// f1 → MAC-A
	macA, _, err = milenageF1(ki, opc, temp, sqn, amf, pc.c1, pc.r1)
	return
}

// milenageResync computes f5* (AK*) and f1* (MAC-S) for SQN re-sync.
// resyncInfo = RAND(16) || AUTS(14); AUTS = (SQN XOR AK*)[6] || MAC-S[8].
func milenageResync(ki, opc, randB [16]byte, auts []byte, pc *profileConstants) (int64, error) {
	if len(auts) != 14 {
		return 0, fmt.Errorf("milenage resync: auts must be 14 bytes, got %d", len(auts))
	}
	temp, err := aesEncrypt(ki, xor16(randB, opc))
	if err != nil {
		return 0, err
	}
	// f5* → AK*  (out[0:6])
	out5s, err := milenageFN(ki, opc, temp, pc.c5, pc.r5)
	if err != nil {
		return 0, err
	}

	// Recover SQN = AUTS[0:6] XOR AK*
	var sqnB [6]byte
	for i := 0; i < 6; i++ {
		sqnB[i] = auts[i] ^ out5s[i]
	}
	sqn := int64(binary.BigEndian.Uint64(append([]byte{0, 0}, sqnB[:]...)))

	// Verify MAC-S
	amf := [2]byte{0x00, 0x00} // AMF=0 for resync
	var sqnU uint64
	for i := 0; i < 6; i++ {
		sqnU = (sqnU << 8) | uint64(sqnB[i])
	}
	_, macS, err := milenageF1(ki, opc, temp, sqnU, amf, pc.c1, pc.r1)
	if err != nil {
		return 0, err
	}
	if !hmac.Equal(macS[:], auts[6:14]) {
		return 0, fmt.Errorf("milenage resync: MAC-S verification failed")
	}
	return sqn, nil
}

// deriveKASME derives KASME from CK, IK, PLMN, and SQN XOR AK per TS 33.401 Annex A.2.
//
//	KASME = HMAC-SHA-256(key=CK||IK, data=0x10 || PLMN(3) || 0x0003 || SQN_XOR_AK(6) || 0x0006)
func deriveKASME(ck, ik [16]byte, plmn []byte, sqnXorAK [6]byte) []byte {
	key := append(ck[:], ik[:]...)
	s := []byte{0x10}
	s = append(s, plmn...)
	s = append(s, 0x00, 0x03)
	s = append(s, sqnXorAK[:]...)
	s = append(s, 0x00, 0x06)
	mac := hmac.New(sha256.New, key)
	mac.Write(s)
	return mac.Sum(nil)
}

// GenerateEUTRANVectorCustom generates a single EUTRAN authentication vector
// using the provided algorithm profile constants.
func GenerateEUTRANVectorCustom(ki, opc, amfB []byte, sqn uint64, plmn []byte, pc *profileConstants) (EUTRANVector, error) {
	if len(ki) != 16 || len(opc) != 16 || len(amfB) != 2 || len(plmn) != 3 {
		return EUTRANVector{}, fmt.Errorf("milenage: invalid input lengths")
	}
	var kiArr, opcArr [16]byte
	copy(kiArr[:], ki)
	copy(opcArr[:], opc)

	var randArr [16]byte
	if _, err := rand.Read(randArr[:]); err != nil {
		return EUTRANVector{}, fmt.Errorf("milenage: rand: %w", err)
	}
	var amfArr [2]byte
	copy(amfArr[:], amfB)

	xres, ck, ik, ak, macA, err := milenageCore(kiArr, opcArr, randArr, sqn, amfArr, pc)
	if err != nil {
		return EUTRANVector{}, err
	}

	// AUTN = (SQN XOR AK)[6] || AMF[2] || MAC-A[8]
	sqnV := sqn
	sqnB := [6]byte{
		byte(sqnV >> 40), byte(sqnV >> 32), byte(sqnV >> 24),
		byte(sqnV >> 16), byte(sqnV >> 8), byte(sqnV),
	}
	var sqnXorAK [6]byte
	for i := 0; i < 6; i++ {
		sqnXorAK[i] = sqnB[i] ^ ak[i]
	}
	autn := append(sqnXorAK[:], amfArr[:]...)
	autn = append(autn, macA[:]...)

	kasme := deriveKASME(ck, ik, plmn, sqnXorAK)

	return EUTRANVector{
		RAND:  randArr[:],
		XRES:  xres[:],
		AUTN:  autn,
		KASME: kasme,
	}, nil
}

// GenerateEAPAKAVectorCustom generates a single EAP-AKA quintuplet using custom profile constants.
func GenerateEAPAKAVectorCustom(ki, opc, amfB []byte, sqn uint64, pc *profileConstants) (EAPAKAVector, error) {
	if len(ki) != 16 || len(opc) != 16 || len(amfB) != 2 {
		return EAPAKAVector{}, fmt.Errorf("milenage: invalid input lengths")
	}
	var kiArr, opcArr [16]byte
	copy(kiArr[:], ki)
	copy(opcArr[:], opc)

	var randArr [16]byte
	if _, err := rand.Read(randArr[:]); err != nil {
		return EAPAKAVector{}, fmt.Errorf("milenage: rand: %w", err)
	}
	var amfArr [2]byte
	copy(amfArr[:], amfB)

	xres, ck, ik, ak, macA, err := milenageCore(kiArr, opcArr, randArr, sqn, amfArr, pc)
	if err != nil {
		return EAPAKAVector{}, err
	}

	sqnV := sqn
	sqnB := [6]byte{
		byte(sqnV >> 40), byte(sqnV >> 32), byte(sqnV >> 24),
		byte(sqnV >> 16), byte(sqnV >> 8), byte(sqnV),
	}
	var sqnXorAK [6]byte
	for i := 0; i < 6; i++ {
		sqnXorAK[i] = sqnB[i] ^ ak[i]
	}
	autn := append(sqnXorAK[:], amfArr[:]...)
	autn = append(autn, macA[:]...)

	return EAPAKAVector{
		RAND: randArr[:],
		XRES: xres[:],
		AUTN: autn,
		CK:   ck[:],
		IK:   ik[:],
	}, nil
}

// ResyncCustom recovers the SQN from a resync AUTS using custom profile constants.
// resyncInfo = RAND(16) || AUTS(14).
func ResyncCustom(ki, opc []byte, resyncInfo []byte, pc *profileConstants) (int64, error) {
	if len(resyncInfo) != 30 {
		return 0, fmt.Errorf("ResyncInfo must be 30 bytes, got %d", len(resyncInfo))
	}
	var kiArr, opcArr, randArr [16]byte
	copy(kiArr[:], ki)
	copy(opcArr[:], opc)
	copy(randArr[:], resyncInfo[:16])
	return milenageResync(kiArr, opcArr, randArr, resyncInfo[16:], pc)
}
