package sbi

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// JWKSStore holds raw JWKS JSON fetched from the NRF.
type JWKSStore struct {
	mu   sync.RWMutex
	json []byte
	keys map[string]crypto.PublicKey
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`

	N string `json:"n"`
	E string `json:"e"`

	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`

	K string `json:"k"`
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

type jwtClaims struct {
	Scope string `json:"scope"`
	Scp   any    `json:"scp"`
	Exp   *int64 `json:"exp"`
	Nbf   *int64 `json:"nbf"`
	Iat   *int64 `json:"iat"`
}

func (j *JWKSStore) Set(raw []byte) error {
	keys, err := parseJWKS(raw)
	if err != nil {
		return err
	}
	j.mu.Lock()
	j.json = append([]byte(nil), raw...)
	j.keys = keys
	j.mu.Unlock()
	return nil
}

func (j *JWKSStore) Get() []byte {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return append([]byte(nil), j.json...)
}

func (j *JWKSStore) Key(kid string) (crypto.PublicKey, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if len(j.keys) == 0 {
		return nil, false
	}
	key, ok := j.keys[kid]
	if ok {
		return key, true
	}
	if kid == "" && len(j.keys) == 1 {
		for _, only := range j.keys {
			return only, true
		}
	}
	return nil, false
}

// OAuthMiddleware validates or bypasses Bearer tokens for inbound SBI requests.
func OAuthMiddleware(enabled, bypass bool, requiredScope string, jwks *JWKSStore, log *zap.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enabled || bypass {
			next.ServeHTTP(w, r)
			return
		}

		meta := RequestMetaFromContext(r.Context())
		baseFields := append(meta.LogFields(),
			zap.String("path", r.URL.Path),
			zap.String("required_scope", requiredScope),
		)

		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			log.Warn("sbi: missing bearer token", baseFields...)
			http.Error(w, `{"error":"missing_token"}`, http.StatusUnauthorized)
			return
		}

		if jwks == nil || jwks.Get() == nil {
			log.Warn("sbi: JWKS not yet available, rejecting request", baseFields...)
			http.Error(w, `{"error":"jwks_unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if token == "" {
			log.Warn("sbi: empty bearer token", baseFields...)
			http.Error(w, `{"error":"missing_token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := validateJWT(token, jwks, requiredScope, time.Now())
		if err != nil {
			log.Warn("sbi: invalid bearer token", append(baseFields, zap.Error(err))...)
			http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
			return
		}

		fields := append(baseFields, zap.Strings("token_scopes", claimScopes(claims)))
		log.Debug("sbi: bearer token validated", fields...)
		next.ServeHTTP(w, r)
	})
}

func validateJWT(token string, jwks *JWKSStore, requiredScope string, now time.Time) (*jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("jwt: expected three parts")
	}

	headerBytes, err := decodeJWTPart(parts[0])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode header: %w", err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("jwt: parse header: %w", err)
	}
	if header.Alg == "" || header.Alg == "none" {
		return nil, errors.New("jwt: unsupported alg")
	}

	payloadBytes, err := decodeJWTPart(parts[1])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode payload: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("jwt: parse claims: %w", err)
	}

	key, ok := jwks.Key(header.Kid)
	if !ok {
		return nil, fmt.Errorf("jwt: no jwk for kid %q", header.Kid)
	}

	signingInput := parts[0] + "." + parts[1]
	sig, err := decodeJWTPart(parts[2])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode signature: %w", err)
	}
	if err := verifyJWTSignature(header.Alg, key, []byte(signingInput), sig); err != nil {
		return nil, err
	}

	if claims.Exp == nil || now.Unix() >= *claims.Exp {
		return nil, errors.New("jwt: token expired")
	}
	if claims.Nbf != nil && now.Unix() < *claims.Nbf {
		return nil, errors.New("jwt: token not yet valid")
	}
	if requiredScope != "" && !hasScope(claims, requiredScope) {
		return nil, fmt.Errorf("jwt: missing required scope %q", requiredScope)
	}

	return &claims, nil
}

func decodeJWTPart(part string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(part)
}

func parseJWKS(raw []byte) (map[string]crypto.PublicKey, error) {
	var doc jwksDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("jwks: parse: %w", err)
	}
	keys := make(map[string]crypto.PublicKey, len(doc.Keys))
	for _, item := range doc.Keys {
		if item.Kid == "" {
			continue
		}
		key, err := parseJWK(item)
		if err != nil {
			return nil, err
		}
		keys[item.Kid] = key
	}
	if len(keys) == 0 {
		return nil, errors.New("jwks: no usable keys")
	}
	return keys, nil
}

func parseJWK(item jwk) (crypto.PublicKey, error) {
	switch item.Kty {
	case "RSA":
		nBytes, err := base64.RawURLEncoding.DecodeString(item.N)
		if err != nil {
			return nil, fmt.Errorf("jwks: invalid RSA modulus: %w", err)
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(item.E)
		if err != nil {
			return nil, fmt.Errorf("jwks: invalid RSA exponent: %w", err)
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 | int(b)
		}
		if e == 0 {
			return nil, errors.New("jwks: invalid RSA exponent")
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: e,
		}, nil
	case "EC":
		curve, err := jwkCurve(item.Crv)
		if err != nil {
			return nil, err
		}
		xBytes, err := base64.RawURLEncoding.DecodeString(item.X)
		if err != nil {
			return nil, fmt.Errorf("jwks: invalid EC x: %w", err)
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(item.Y)
		if err != nil {
			return nil, fmt.Errorf("jwks: invalid EC y: %w", err)
		}
		pub := &ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}
		if !curve.IsOnCurve(pub.X, pub.Y) {
			return nil, errors.New("jwks: EC point not on curve")
		}
		return pub, nil
	case "OKP":
		if item.Crv != "Ed25519" {
			return nil, fmt.Errorf("jwks: unsupported OKP curve %q", item.Crv)
		}
		k, err := base64.RawURLEncoding.DecodeString(item.X)
		if err != nil {
			return nil, fmt.Errorf("jwks: invalid OKP x: %w", err)
		}
		if len(k) != ed25519.PublicKeySize {
			return nil, errors.New("jwks: invalid Ed25519 key length")
		}
		return ed25519.PublicKey(k), nil
	default:
		return nil, fmt.Errorf("jwks: unsupported kty %q", item.Kty)
	}
}

func jwkCurve(crv string) (elliptic.Curve, error) {
	switch crv {
	case "P-256":
		return elliptic.P256(), nil
	case "P-384":
		return elliptic.P384(), nil
	case "P-521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("jwks: unsupported EC curve %q", crv)
	}
}

func verifyJWTSignature(alg string, key crypto.PublicKey, signingInput, signature []byte) error {
	switch alg {
	case "RS256":
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for RS256")
		}
		sum := sha256.Sum256(signingInput)
		return rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], signature)
	case "RS384":
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for RS384")
		}
		sum := sha512.Sum384(signingInput)
		return rsa.VerifyPKCS1v15(pub, crypto.SHA384, sum[:], signature)
	case "RS512":
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for RS512")
		}
		sum := sha512.Sum512(signingInput)
		return rsa.VerifyPKCS1v15(pub, crypto.SHA512, sum[:], signature)
	case "PS256":
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for PS256")
		}
		sum := sha256.Sum256(signingInput)
		return rsa.VerifyPSS(pub, crypto.SHA256, sum[:], signature, nil)
	case "PS384":
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for PS384")
		}
		sum := sha512.Sum384(signingInput)
		return rsa.VerifyPSS(pub, crypto.SHA384, sum[:], signature, nil)
	case "PS512":
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for PS512")
		}
		sum := sha512.Sum512(signingInput)
		return rsa.VerifyPSS(pub, crypto.SHA512, sum[:], signature, nil)
	case "ES256":
		pub, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for ES256")
		}
		sum := sha256.Sum256(signingInput)
		return verifyECDSA(pub, sum[:], signature)
	case "ES384":
		pub, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for ES384")
		}
		sum := sha512.Sum384(signingInput)
		return verifyECDSA(pub, sum[:], signature)
	case "ES512":
		pub, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for ES512")
		}
		sum := sha512.Sum512(signingInput)
		return verifyECDSA(pub, sum[:], signature)
	case "EdDSA":
		pub, ok := key.(ed25519.PublicKey)
		if !ok {
			return errors.New("jwt: key type mismatch for EdDSA")
		}
		if !ed25519.Verify(pub, signingInput, signature) {
			return errors.New("jwt: invalid EdDSA signature")
		}
		return nil
	default:
		return fmt.Errorf("jwt: unsupported alg %q", alg)
	}
}

func verifyECDSA(pub *ecdsa.PublicKey, digest, signature []byte) error {
	size := (pub.Curve.Params().BitSize + 7) / 8
	if len(signature) != size*2 {
		return errors.New("jwt: invalid ECDSA signature length")
	}
	r := new(big.Int).SetBytes(signature[:size])
	s := new(big.Int).SetBytes(signature[size:])
	if !ecdsa.Verify(pub, digest, r, s) {
		return errors.New("jwt: invalid ECDSA signature")
	}
	return nil
}

func hasScope(claims jwtClaims, required string) bool {
	for _, scope := range claimScopes(&claims) {
		if subtle.ConstantTimeCompare([]byte(scope), []byte(required)) == 1 {
			return true
		}
	}
	return false
}

func claimScopes(claims *jwtClaims) []string {
	if claims == nil {
		return nil
	}
	var scopes []string
	for _, scope := range strings.Fields(claims.Scope) {
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	switch v := claims.Scp.(type) {
	case string:
		for _, scope := range strings.Fields(v) {
			if scope != "" {
				scopes = append(scopes, scope)
			}
		}
	case []any:
		for _, item := range v {
			if scope, ok := item.(string); ok && scope != "" {
				scopes = append(scopes, scope)
			}
		}
	}
	return scopes
}
