package adapter

import (
	"bytes"
	"database/sql/driver"
	"testing"
)

const testCipherKey = "test-key-do-not-use-in-prod-019-column-encryption"

func newTestEngine(t *testing.T) *CipherEngine {
	t.Helper()
	eng, err := NewCipherEngine([]byte(testCipherKey))
	if err != nil {
		t.Fatalf("NewCipherEngine: %v", err)
	}
	return eng
}

func TestCipherEngine_RoundTripString(t *testing.T) {
	eng := newTestEngine(t)
	inputs := []string{
		"alice@example.com",
		"",
		"unicode-中文-🔐-test",
		"a",
		string(bytes.Repeat([]byte("x"), 4096)),
	}
	for _, in := range inputs {
		ct, err := eng.EncryptString(in)
		if err != nil {
			t.Fatalf("EncryptString(%q): %v", in, err)
		}
		if in != "" && bytes.Equal(ct, []byte(in)) {
			t.Fatalf("ciphertext equals plaintext for %q (no encryption happened)", in)
		}
		got, err := eng.DecryptString(ct)
		if err != nil {
			t.Fatalf("DecryptString(%q): %v", in, err)
		}
		if got != in {
			t.Fatalf("round-trip mismatch: in=%q got=%q", in, got)
		}
	}
}

func TestCipherEngine_NonceUniqueness(t *testing.T) {
	eng := newTestEngine(t)
	ct1, err := eng.EncryptString("repeat")
	if err != nil {
		t.Fatal(err)
	}
	ct2, err := eng.EncryptString("repeat")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ct1, ct2) {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext (nonce reuse)")
	}
}

func TestCipherEngine_DecryptDetectsTampering(t *testing.T) {
	eng := newTestEngine(t)
	ct, err := eng.EncryptString("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	// Flip one byte in the body of the sealed blob.
	tampered := append([]byte(nil), ct...)
	tampered[len(tampered)-1] ^= 0xFF
	if _, err := eng.DecryptString(tampered); err == nil {
		t.Fatal("expected decryption to fail on tampered ciphertext, got nil")
	}
}

func TestCipherEngine_KeyIsolation(t *testing.T) {
	a, err := NewCipherEngine([]byte("key-A"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewCipherEngine([]byte("key-B"))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := a.EncryptString("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.DecryptString(ct); err == nil {
		t.Fatal("expected decryption with a different key to fail")
	}
}

func TestCipherEngine_RejectsEmptyKey(t *testing.T) {
	if _, err := NewCipherEngine(nil); err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestEncryptedField_ScanValueRoundTrip(t *testing.T) {
	eng := newTestEngine(t)
	original := "bob@example.com"

	var field EncryptedField
	field.Engine = eng
	field.Plain = []byte(original)

	dv, err := field.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	raw, ok := dv.([]byte)
	if !ok {
		t.Fatalf("Value returned %T, want []byte", dv)
	}
	if bytes.Equal(raw, []byte(original)) {
		t.Fatal("driver value equals plaintext (encryption skipped)")
	}

	var got EncryptedField
	got.Engine = eng
	if err := got.Scan(raw); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got.String() != original {
		t.Fatalf("decrypted value mismatch: got %q want %q", got.String(), original)
	}
}

func TestEncryptedField_ScanNilIsNil(t *testing.T) {
	eng := newTestEngine(t)
	var f EncryptedField
	f.Engine = eng
	if err := f.Scan(nil); err != nil {
		t.Fatalf("Scan(nil): %v", err)
	}
	if f.Plain != nil || f.Cipher != nil {
		t.Fatalf("expected both Plain and Cipher nil, got Plain=%v Cipher=%v", f.Plain, f.Cipher)
	}
}

func TestEncryptedField_ValueEmptyIsNil(t *testing.T) {
	eng := newTestEngine(t)
	f := EncryptedField{Engine: eng}
	dv, err := f.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if dv != nil {
		t.Fatalf("Value for empty Plain: got %v, want nil", dv)
	}
}

func TestEncryptedField_ValueRequiresEngine(t *testing.T) {
	f := EncryptedField{Plain: []byte("x")}
	_, err := f.Value()
	if err == nil {
		t.Fatal("expected error when encrypting without an engine")
	}
}

func TestEncryptedField_ScanRejectsUnsupportedType(t *testing.T) {
	eng := newTestEngine(t)
	var f EncryptedField
	f.Engine = eng
	err := f.Scan(123)
	if err == nil {
		t.Fatal("expected error for unsupported scan type")
	}
}

func TestEncodeDecodeBase64(t *testing.T) {
	in := []byte{0x00, 0x01, 0x02, 0xfe, 0xff}
	s := EncodeBase64(in)
	out, err := DecodeBase64(s)
	if err != nil {
		t.Fatalf("DecodeBase64: %v", err)
	}
	if !bytes.Equal(in, out) {
		t.Fatalf("base64 round-trip mismatch: %v vs %v", in, out)
	}
}

// Compile-time check that EncryptedField satisfies driver.Valuer and the
// sql.Scanner shape we rely on.
var _ driver.Valuer = EncryptedField{}

func TestSetDefaultEngineRoundTrip(t *testing.T) {
	eng := newTestEngine(t)
	SetDefaultEngine(eng)
	defer SetDefaultEngine(nil)

	if DefaultEngine() == nil {
		t.Fatal("DefaultEngine returned nil after SetDefaultEngine")
	}
	ct, err := DefaultEngine().EncryptString("hello")
	if err != nil {
		t.Fatal(err)
	}
	got, err := DefaultEngine().DecryptString(ct)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("DefaultEngine round-trip: got %q want %q", got, "hello")
	}
}

func TestNewCipherEngine_StretchesShortKey(t *testing.T) {
	// A 5-byte key should be stretched via SHA-256 into a 32-byte AES key.
	a, err := NewCipherEngine([]byte("short"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewCipherEngine([]byte("short"))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := a.EncryptString("ok")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.DecryptString(ct); err != nil {
		t.Fatalf("two engines built from the same short key should interoperate: %v", err)
	}
}

func TestNewCipherEngine_AcceptsExactKeySizes(t *testing.T) {
	for _, n := range []int{16, 24, 32} {
		key := bytes.Repeat([]byte{0xAB}, n)
		if _, err := NewCipherEngine(key); err != nil {
			t.Fatalf("NewCipherEngine(%d-byte key): %v", n, err)
		}
	}
}

func TestEncryptedField_ScanSupportsString(t *testing.T) {
	eng := newTestEngine(t)
	ct, err := eng.EncryptString("via-string")
	if err != nil {
		t.Fatal(err)
	}
	var f EncryptedField
	f.Engine = eng
	if err := f.Scan(string(ct)); err != nil {
		t.Fatalf("Scan(string): %v", err)
	}
	if f.String() != "via-string" {
		t.Fatalf("Scan(string) decrypted value: got %q", f.String())
	}
}

func TestCipherEngine_DecryptRejectsTruncated(t *testing.T) {
	eng := newTestEngine(t)
	if _, err := eng.DecryptString([]byte{0x01, 0x02}); err == nil {
		t.Fatal("expected error for ciphertext shorter than nonce")
	}
}

func TestCipherEngine_DecryptWrongKey(t *testing.T) {
	a, _ := NewCipherEngine([]byte("alpha"))
	b, _ := NewCipherEngine([]byte("bravo"))
	ct, err := a.EncryptString("payload")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.DecryptString(ct); err == nil {
		t.Fatal("expected decrypt to fail with wrong key")
	}
}
