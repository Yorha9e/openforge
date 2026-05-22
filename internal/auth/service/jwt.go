package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Claims struct {
	UserID    string `json:"uid"`
	Role      string `json:"role"`
	ProjectID string `json:"pid,omitempty"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type JWTService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTService(secret string, accessTTL, refreshTTL time.Duration) *JWTService {
	return &JWTService{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (s *JWTService) Issue(userID, role, projectID string) (*TokenPair, error) {
	now := time.Now().UTC()
	access, err := s.encode(Claims{
		UserID: userID, Role: role, ProjectID: projectID,
		IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(s.accessTTL).UnixMilli(),
	})
	if err != nil {
		return nil, err
	}
	refresh, err := s.encode(Claims{
		UserID: userID, Role: role,
		IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(s.refreshTTL).UnixMilli(),
	})
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken: access, RefreshToken: refresh,
		ExpiresIn: int64(s.accessTTL.Seconds()),
	}, nil
}

func (s *JWTService) Verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}
	if time.Now().UTC().UnixMilli() > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}
	expectedSig := s.sign(parts[0] + "." + parts[1])
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}
	return &claims, nil
}

func (s *JWTService) encode(claims Claims) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := s.sign(header + "." + payload)
	return header + "." + payload + "." + sig, nil
}

func (s *JWTService) sign(data string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
