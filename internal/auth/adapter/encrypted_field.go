package adapter

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

// CipherEngine is the application-wide symmetric cipher used for column-level
// PII encryption (item #20). The key is supplied at startup from
// configuration (typically sourced from Vault via app.encryption_key).
//
// The on-disk wire format is: base64(nonce || ciphertext || gcm_tag).
type CipherEngine struct {
	gcm cipher.AEAD
}

// NewCipherEngine builds a CipherEngine from a raw key. Acceptable key lengths
// are 16, 24, or 32 bytes; shorter inputs are stretched via SHA-256 to fit the
// nearest AES key size so callers can pass short shared secrets if they wish.
func NewCipherEngine(key []byte) (*CipherEngine, error) {
	if len(key) == 0 {
		return nil, errors.New("cipher: empty key")
	}
	material := key
	switch len(material) {
	case 16, 24, 32:
		// already a valid AES key size
	default:
		// Derive a 32-byte key from arbitrary input.
		sum := sha256.Sum256(material)
		material = sum[:]
	}
	block, err := aes.NewCipher(material)
	if err != nil {
		return nil, fmt.Errorf("cipher: aes.NewCipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher: cipher.NewGCM: %w", err)
	}
	return &CipherEngine{gcm: gcm}, nil
}

// Encrypt seals plaintext and returns a self-contained byte slice containing
// a fresh random nonce prepended to the GCM ciphertext+tag.
func (e *CipherEngine) Encrypt(plaintext []byte) ([]byte, error) {
	if e == nil || e.gcm == nil {
		return nil, errors.New("cipher: engine not initialized")
	}
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("cipher: read nonce: %w", err)
	}
	sealed := e.gcm.Seal(nonce, nonce, plaintext, nil)
	return sealed, nil
}

// Decrypt inverts Encrypt. It returns an error if the ciphertext has been
// tampered with or was sealed under a different key.
func (e *CipherEngine) Decrypt(ciphertext []byte) ([]byte, error) {
	if e == nil || e.gcm == nil {
		return nil, errors.New("cipher: engine not initialized")
	}
	ns := e.gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("cipher: ciphertext too short")
	}
	nonce, body := ciphertext[:ns], ciphertext[ns:]
	plain, err := e.gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return nil, fmt.Errorf("cipher: gcm open: %w", err)
	}
	return plain, nil
}

// EncryptString is a convenience wrapper.
func (e *CipherEngine) EncryptString(s string) ([]byte, error) {
	return e.Encrypt([]byte(s))
}

// DecryptString is a convenience wrapper.
func (e *CipherEngine) DecryptString(b []byte) (string, error) {
	plain, err := e.Decrypt(b)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// EncodeBase64 returns the canonical transport form for an EncryptedField.
func EncodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// DecodeBase64 parses the transport form produced by EncodeBase64.
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// EncryptedField is a sql.Scanner / driver.Valuer for BYTEA columns holding
// column-level-encrypted payloads. When scanned, the engine transparently
// decrypts; when written, the engine transparently encrypts. It transparently
// passes through nil so the underlying column can be NULL.
type EncryptedField struct {
	Engine *CipherEngine
	// Plain holds the decrypted value when read from the DB and the value to
	// be encrypted when written. Callers should treat this as opaque; use the
	// String/Bytes helpers.
	Plain []byte
	// Cipher is the on-the-wire ciphertext. It is populated by Scan and used
	// (re-encrypted via Engine) by Value. External callers normally only set
	// Plain.
	Cipher []byte
}

// Scan implements sql.Scanner. Accepts []byte (raw BYTEA) or string.
func (e *EncryptedField) Scan(src any) error {
	if e == nil {
		return errors.New("cipher: nil receiver")
	}
	if src == nil {
		e.Plain = nil
		e.Cipher = nil
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("cipher: unsupported scan type %T", src)
	}
	e.Cipher = append(e.Cipher[:0], raw...)
	if e.Engine == nil {
		// No engine registered: keep ciphertext in Plain so callers can still
		// see what was on disk (useful for debugging).
		e.Plain = append(e.Plain[:0], raw...)
		return nil
	}
	plain, err := e.Engine.Decrypt(raw)
	if err != nil {
		return err
	}
	e.Plain = plain
	return nil
}

// Value implements driver.Valuer. Returns nil for an empty Plain.
func (e EncryptedField) Value() (driver.Value, error) {
	if len(e.Plain) == 0 {
		return nil, nil
	}
	if e.Engine == nil {
		return nil, errors.New("cipher: engine not set on EncryptedField; cannot encrypt")
	}
	sealed, err := e.Engine.Encrypt(e.Plain)
	if err != nil {
		return nil, err
	}
	return sealed, nil
}

// String returns the decrypted plaintext as a string (empty if nil).
func (e EncryptedField) String() string {
	return string(e.Plain)
}

// defaultEngine is a process-wide fallback used only by tests. Production code
// must inject a real engine via SetDefaultEngine.
var (
	defaultEngineMu sync.RWMutex
	defaultEngine   *CipherEngine
)

// SetDefaultEngine registers a process-wide fallback engine. Passing nil
// clears the fallback. Intended for boot wiring; tests may also call this.
func SetDefaultEngine(e *CipherEngine) {
	defaultEngineMu.Lock()
	defaultEngine = e
	defaultEngineMu.Unlock()
}

// DefaultEngine returns the registered fallback engine, or nil.
func DefaultEngine() *CipherEngine {
	defaultEngineMu.RLock()
	defer defaultEngineMu.RUnlock()
	return defaultEngine
}
