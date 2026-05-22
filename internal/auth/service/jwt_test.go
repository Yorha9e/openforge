package service

import (
	"testing"
	"time"
)

func TestJWTIssueAndVerify(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	svc := NewJWTService(secret, 1*time.Hour, 24*time.Hour)
	token, err := svc.Issue("user@test.com", "pm", "proj-001")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if token.AccessToken == "" {
		t.Fatal("AccessToken is empty")
	}
	if token.RefreshToken == "" {
		t.Fatal("RefreshToken is empty")
	}
	claims, err := svc.Verify(token.AccessToken)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims.UserID != "user@test.com" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user@test.com")
	}
	if claims.Role != "pm" {
		t.Errorf("Role = %q, want %q", claims.Role, "pm")
	}
}

func TestJWTVerifyExpired(t *testing.T) {
	svc := NewJWTService("test-secret", 1*time.Millisecond, 24*time.Hour)
	token, _ := svc.Issue("u@t.com", "dev", "p1")
	time.Sleep(5 * time.Millisecond)
	_, err := svc.Verify(token.AccessToken)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestJWTVerifyInvalidSignature(t *testing.T) {
	svc := NewJWTService("real-secret", 1*time.Hour, 24*time.Hour)
	token, _ := svc.Issue("u@t.com", "dev", "p1")
	otherSvc := NewJWTService("wrong-secret", 1*time.Hour, 24*time.Hour)
	_, err := otherSvc.Verify(token.AccessToken)
	if err == nil {
		t.Fatal("expected error for wrong signature")
	}
}

func TestJWTVerifyTampered(t *testing.T) {
	svc := NewJWTService("secret", 1*time.Hour, 24*time.Hour)
	token, _ := svc.Issue("u@t.com", "dev", "p1")
	tampered := token.AccessToken + "x"
	_, err := svc.Verify(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}
