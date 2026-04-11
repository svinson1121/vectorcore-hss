package udm

// hnet.go — Home Network SUCI decryption (TS 33.501 Annex C).
//
// Supports:
//   Profile A (scheme 1): X25519-ECDH + HKDF-SHA256 + AES-128-CTR + HMAC-SHA256-64
//   Profile B (scheme 2): P-256-ECDH  + HKDF-SHA256 + AES-128-CTR + HMAC-SHA256-64
//
// Key derivation (both profiles, TS 33.501 Annex C.3.2 / C.4):
//   Z         = ECDH(hn_private_key, eph_public_key)
//   OKM       = HKDF-SHA256(IKM=Z, salt=eph_public_key, info="", L=64)
//   enc_key   = OKM[0:16]
//   icb       = OKM[16:32]   (AES-CTR initial counter block)
//   mac_key   = OKM[32:64]
//
// Scheme output layout (hex-encoded in SUCI string):
//   Profile A: eph_pub(32B) || enc_msin(?B) || mac(8B)
//   Profile B: eph_pub(33B compressed) || enc_msin(?B) || mac(8B)
//
// MSIN encoding: packed BCD (semi-octet), odd-length padded with trailing 0xF nibble.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"

	xhkdf "golang.org/x/crypto/hkdf"

	"github.com/svinson1121/vectorcore-hss/internal/config"
)

const (
	suciProfileA = 1 // X25519
	suciProfileB = 2 // P-256

	ephPubLenA  = 32 // X25519 public key
	ephPubLenB  = 33 // P-256 compressed public key
	macTagLen   = 8  // HMAC-SHA256 truncated to 64 bits
	hkdfOutLen  = 64 // enc_key(16) + icb(16) + mac_key(32)
)

// HNetKeyStore holds loaded private keys keyed by HomeNetworkPublicKeyIdentifier.
type HNetKeyStore struct {
	keys map[int]*hnetKey
}

type hnetKey struct {
	scheme int
	priv   *ecdh.PrivateKey
}

// LoadHNetKeys loads all configured Home Network private keys from PEM files.
// Returns an empty (non-nil) store if cfgKeys is empty — callers can always call Decrypt.
func LoadHNetKeys(cfgKeys []config.HNetKeyConfig) (*HNetKeyStore, error) {
	store := &HNetKeyStore{keys: make(map[int]*hnetKey)}
	for _, c := range cfgKeys {
		if c.KeyID < 1 || c.KeyID > 255 {
			return nil, fmt.Errorf("hnet: key_id %d out of range (must be 1-255)", c.KeyID)
		}
		if c.Scheme != suciProfileA && c.Scheme != suciProfileB {
			return nil, fmt.Errorf("hnet: key_id %d has unsupported scheme %d (must be 1 or 2)", c.KeyID, c.Scheme)
		}
		priv, err := loadECPrivateKey(c.KeyFile, c.Scheme)
		if err != nil {
			return nil, fmt.Errorf("hnet: key_id %d: %w", c.KeyID, err)
		}
		store.keys[c.KeyID] = &hnetKey{scheme: c.Scheme, priv: priv}
	}
	return store, nil
}

// Decrypt runs ECIES decryption for the given scheme, key ID, and scheme output bytes.
// Returns the decoded MSIN string on success.
func (s *HNetKeyStore) Decrypt(scheme, keyID int, schemeOutput []byte) (string, error) {
	if scheme != suciProfileA && scheme != suciProfileB {
		return "", fmt.Errorf("udm: encrypted SUCI scheme %d not supported", scheme)
	}
	k, ok := s.keys[keyID]
	if !ok {
		return "", fmt.Errorf("udm: no Home Network key loaded for key_id %d", keyID)
	}
	if k.scheme != scheme {
		return "", fmt.Errorf("udm: key_id %d is scheme %d, SUCI has scheme %d", keyID, k.scheme, scheme)
	}

	ephLen := ephPubLenA
	if scheme == suciProfileB {
		ephLen = ephPubLenB
	}
	if len(schemeOutput) < ephLen+macTagLen {
		return "", errors.New("udm: SUCI scheme output too short")
	}

	ephPubBytes := schemeOutput[:ephLen]
	macTag := schemeOutput[len(schemeOutput)-macTagLen:]
	encMSIN := schemeOutput[ephLen : len(schemeOutput)-macTagLen]

	// ECDH
	var curve ecdh.Curve
	if scheme == suciProfileA {
		curve = ecdh.X25519()
	} else {
		curve = ecdh.P256()
	}
	ephPub, err := curve.NewPublicKey(ephPubBytes)
	if err != nil {
		return "", fmt.Errorf("udm: parse ephemeral public key: %w", err)
	}
	sharedSecret, err := k.priv.ECDH(ephPub)
	if err != nil {
		return "", fmt.Errorf("udm: ECDH failed: %w", err)
	}

	// HKDF-SHA256: IKM=sharedSecret, salt=ephPubBytes, info="", L=64
	hkdfReader := xhkdf.New(sha256.New, sharedSecret, ephPubBytes, nil)
	okm := make([]byte, hkdfOutLen)
	if _, err := io.ReadFull(hkdfReader, okm); err != nil {
		return "", fmt.Errorf("udm: HKDF failed: %w", err)
	}
	encKey := okm[0:16]
	icb := okm[16:32]
	macKey := okm[32:64]

	// Verify MAC: HMAC-SHA256(mac_key, enc_msin)[0:8]
	h := hmac.New(sha256.New, macKey)
	h.Write(encMSIN)
	expectedMAC := h.Sum(nil)[:macTagLen]
	if !hmac.Equal(expectedMAC, macTag) {
		return "", errors.New("udm: SUCI MAC verification failed")
	}

	// Decrypt: AES-128-CTR(enc_key, icb, enc_msin)
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return "", fmt.Errorf("udm: AES cipher: %w", err)
	}
	msinBytes := make([]byte, len(encMSIN))
	cipher.NewCTR(block, icb).XORKeyStream(msinBytes, encMSIN)

	return decodeMSINBCD(msinBytes), nil
}

// PublicKeyHex returns the hex-encoded public key for a given KeyID.
// Useful for displaying what needs to be provisioned on the SIM.
func (s *HNetKeyStore) PublicKeyHex(keyID int) (string, error) {
	k, ok := s.keys[keyID]
	if !ok {
		return "", fmt.Errorf("hnet: no key loaded for key_id %d", keyID)
	}
	return fmt.Sprintf("%x", k.priv.PublicKey().Bytes()), nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// loadECPrivateKey reads a PEM-encoded EC private key for the given SUCI scheme.
//
// Handles the formats produced by Open5GS and standard openssl:
//   curve25519-N.key  — single PKCS#8 block  (BEGIN PRIVATE KEY)
//   secp256r1-N.key   — EC PARAMETERS block followed by SEC1 block (BEGIN EC PRIVATE KEY)
//
// Iterates all PEM blocks so that an EC PARAMETERS preamble is skipped cleanly.
func loadECPrivateKey(path string, scheme int) (*ecdh.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file %q: %w", path, err)
	}

	// Collect all PEM blocks — secp256r1 keys ship with an EC PARAMETERS preamble.
	var blocks []*pem.Block
	rest := data
	for {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		blocks = append(blocks, b)
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no PEM block found in %q", path)
	}

	for _, block := range blocks {
		switch block.Type {
		case "PRIVATE KEY": // PKCS#8 — openssl genpkey (X25519 and P-256)
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse PKCS8 key in %q: %w", path, err)
			}
			return toECDHKey(key, scheme, path)

		case "EC PRIVATE KEY": // SEC1 — openssl ecparam -genkey (P-256)
			ecKey, err := x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse SEC1 EC key in %q: %w", path, err)
			}
			return ecKey.ECDH()
		}
		// "EC PARAMETERS" and other blocks are skipped.
	}

	// Last resort: raw 32-byte X25519 private scalar in a non-standard PEM wrapper.
	if scheme == suciProfileA && len(blocks[0].Bytes) == 32 {
		return ecdh.X25519().NewPrivateKey(blocks[0].Bytes)
	}

	return nil, fmt.Errorf("no recognised private key block in %q", path)
}

// toECDHKey converts a parsed PKCS#8 private key to *ecdh.PrivateKey,
// asserting the curve matches the expected SUCI scheme.
func toECDHKey(key any, scheme int, path string) (*ecdh.PrivateKey, error) {
	var expectedCurve ecdh.Curve
	if scheme == suciProfileA {
		expectedCurve = ecdh.X25519()
	} else {
		expectedCurve = ecdh.P256()
	}

	// Go 1.20+ returns *ecdh.PrivateKey for X25519; *ecdsa.PrivateKey for P-256.
	switch k := key.(type) {
	case *ecdh.PrivateKey:
		if k.Curve() != expectedCurve {
			return nil, fmt.Errorf("key in %q is wrong curve for scheme %d", path, scheme)
		}
		return k, nil
	case interface{ ECDH() (*ecdh.PrivateKey, error) }: // *ecdsa.PrivateKey
		ecdhKey, err := k.ECDH()
		if err != nil {
			return nil, fmt.Errorf("convert ecdsa→ecdh in %q: %w", path, err)
		}
		if ecdhKey.Curve() != expectedCurve {
			return nil, fmt.Errorf("key in %q is wrong curve for scheme %d", path, scheme)
		}
		return ecdhKey, nil
	default:
		return nil, fmt.Errorf("unsupported key type %T in %q", key, path)
	}
}

// decodeMSINBCD converts packed BCD bytes back to a digit string.
// A trailing 0xF nibble is the odd-length padding sentinel and is dropped.
func decodeMSINBCD(b []byte) string {
	digits := make([]byte, 0, len(b)*2)
	for i, by := range b {
		hi := (by >> 4) & 0x0F
		lo := by & 0x0F
		digits = append(digits, '0'+hi)
		// Trailing 0xF in the low nibble of the last byte = padding.
		if i == len(b)-1 && lo == 0x0F {
			break
		}
		digits = append(digits, '0'+lo)
	}
	return string(digits)
}
