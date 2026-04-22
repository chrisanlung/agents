package helper

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"

	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

// TokenIssuer is the interface the service layer depends on for JWT operations.
type TokenIssuer interface {
	IssueAccessToken(ctx context.Context, claims model.AccessClaims) (string, error)
	VerifyAccessToken(ctx context.Context, tokenString string) (model.AccessClaims, error)
	JWKS(ctx context.Context) ([]byte, error)
}

// KeyPair holds the loaded RSA key pair used for JWT signing.
type KeyPair struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	// KeyID is the stable identifier embedded in the JWT kid header.
	KeyID string
}

// LoadOrGenerateKeyPair loads the RSA private key from the given path.
// If the path is empty or the file is missing, an ephemeral 2048-bit key pair
// is generated for development and a loud warning is written to stderr.
// Production deployments MUST supply a real key path.
func LoadOrGenerateKeyPair(_ context.Context, privateKeyPath string) (*KeyPair, error) {
	if privateKeyPath != "" {
		kp, err := loadKeyFromFile(privateKeyPath)
		if err == nil {
			return kp, nil
		}
		fmt.Fprintf(os.Stderr, "WARN: failed to load JWT private key from %s: %v — falling back to ephemeral key\n", privateKeyPath, err)
	}

	fmt.Fprintln(os.Stderr, "WARN: using ephemeral RSA key pair — JWTs will not survive a restart. Set JWT_PRIVATE_KEY_PATH for production.")
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral RSA key: %w", err)
	}
	return &KeyPair{
		PrivateKey: privKey,
		PublicKey:  &privKey.PublicKey,
		KeyID:      "dev-ephemeral",
	}, nil
}

func loadKeyFromFile(path string) (*KeyPair, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from operator config
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in key file")
	}
	var privKey *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		privKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("parse PKCS8 key: %w", parseErr)
		}
		var ok bool
		privKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not RSA")
		}
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("parse RSA key: %w", err)
	}
	return &KeyPair{
		PrivateKey: privKey,
		PublicKey:  &privKey.PublicKey,
		KeyID:      "primary",
	}, nil
}

// lustiaCustomClaims extends jwt.RegisteredClaims with Lustia-specific fields.
// scope and membership_id are added in ADR 0007 (migration 000009).
type lustiaCustomClaims struct {
	jwt.RegisteredClaims
	Scope              string   `json:"scope"`
	TenantID           string   `json:"tenant_id"`
	MembershipID       *string  `json:"membership_id"`
	Roles              []string `json:"roles"`
	Permissions        []string `json:"permissions"`
	Branches           []string `json:"branches"`
	Email              string   `json:"email"`
	FullName           string   `json:"full_name"`
	MustChangePassword bool     `json:"must_change_password,omitempty"`
}

// JWTIssuer implements TokenIssuer using RS256.
type JWTIssuer struct {
	kp *KeyPair
}

// NewJWTIssuer constructs a JWTIssuer with the given key pair.
func NewJWTIssuer(kp *KeyPair) *JWTIssuer { return &JWTIssuer{kp: kp} }

// IssueAccessToken creates a signed RS256 JWT for the given claims.
func (j *JWTIssuer) IssueAccessToken(_ context.Context, claims model.AccessClaims) (string, error) {
	jwtClaims := lustiaCustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    claims.Issuer,
			Subject:   claims.Subject,
			Audience:  jwt.ClaimStrings(claims.Audience),
			IssuedAt:  jwt.NewNumericDate(claims.IssuedAt),
			ExpiresAt: jwt.NewNumericDate(claims.ExpiresAt),
			ID:        claims.JWTID,
		},
		Scope:              string(claims.Scope),
		TenantID:           claims.TenantID,
		MembershipID:       claims.MembershipID,
		Roles:              claims.Roles,
		Permissions:        claims.Permissions,
		Branches:           claims.Branches,
		Email:              claims.Email,
		FullName:           claims.FullName,
		MustChangePassword: claims.MustChangePassword,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwtClaims)
	token.Header["kid"] = j.kp.KeyID

	signed, err := token.SignedString(j.kp.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return signed, nil
}

// VerifyAccessToken parses and validates a signed JWT string.
func (j *JWTIssuer) VerifyAccessToken(_ context.Context, tokenString string) (model.AccessClaims, error) {
	var custom lustiaCustomClaims
	token, err := jwt.ParseWithClaims(tokenString, &custom, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.kp.PublicKey, nil
	})
	if err != nil {
		return model.AccessClaims{}, fmt.Errorf("parse JWT: %w", err)
	}
	if !token.Valid {
		return model.AccessClaims{}, fmt.Errorf("invalid JWT")
	}
	return model.AccessClaims{
		Issuer:             custom.Issuer,
		Subject:            custom.Subject,
		Audience:           []string(custom.Audience),
		IssuedAt:           custom.IssuedAt.Time,
		ExpiresAt:          custom.ExpiresAt.Time,
		JWTID:              custom.ID,
		Scope:              model.TokenScope(custom.Scope),
		TenantID:           custom.TenantID,
		MembershipID:       custom.MembershipID,
		Roles:              custom.Roles,
		Permissions:        custom.Permissions,
		Branches:           custom.Branches,
		Email:              custom.Email,
		FullName:           custom.FullName,
		MustChangePassword: custom.MustChangePassword,
	}, nil
}

// JWKS returns the JSON Web Key Set document for the current public key.
func (j *JWTIssuer) JWKS(_ context.Context) ([]byte, error) {
	pub := j.kp.PublicKey
	jwk := rsaPublicKeyToJWK(pub, j.kp.KeyID)
	doc := map[string]interface{}{"keys": []interface{}{jwk}}
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal JWKS: %w", err)
	}
	return b, nil
}

func rsaPublicKeyToJWK(pub *rsa.PublicKey, kid string) map[string]interface{} {
	return map[string]interface{}{
		"kty": "RSA",
		"use": "sig",
		"alg": "RS256",
		"kid": kid,
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}
