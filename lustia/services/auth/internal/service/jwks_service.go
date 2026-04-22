package service

import (
	"context"
	"encoding/json"
	"fmt"
)

// JWKSService returns the public JSON Web Key Set document so downstream
// services can verify access tokens without calling the auth service on
// every request.
type JWKSService struct {
	issuer TokenIssuer
}

// NewJWKSService constructs a JWKSService.
func NewJWKSService(issuer TokenIssuer) *JWKSService { return &JWKSService{issuer: issuer} }

// GetJWKS returns the JWKS document as a raw JSON message.
func (s *JWKSService) GetJWKS(ctx context.Context) (json.RawMessage, error) {
	raw, err := s.issuer.JWKS(ctx)
	if err != nil {
		return nil, fmt.Errorf("get jwks: %w", err)
	}
	return json.RawMessage(raw), nil
}
