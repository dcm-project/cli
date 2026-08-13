package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

const keyringService = "dcm-cli"

// TokenData holds the tokens and cached OIDC metadata from a login session.
type TokenData struct {
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token"`
	IDToken       string    `json:"id_token,omitempty"`
	Expiry        time.Time `json:"expiry"`
	TokenEndpoint string    `json:"token_endpoint"`
}

func (t *TokenData) String() string {
	return "[REDACTED]"
}

// IsExpired checks whether the access token has expired. It prefers the
// unverified JWT exp claim, falling back to TokenData.Expiry for opaque
// tokens. The clockSkew parameter provides a buffer for clock differences.
func (t *TokenData) IsExpired(clockSkew time.Duration) bool {
	exp, err := jwtExpiry(t.AccessToken)
	if err != nil {
		if t.Expiry.IsZero() {
			return true
		}
		exp = t.Expiry
	}
	return time.Now().After(exp.Add(-clockSkew))
}

// jwtExpiry extracts the exp claim from a JWT without signature verification.
func jwtExpiry(token string) (time.Time, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid JWT: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("decoding JWT payload: %w", err)
	}

	var claims struct {
		Exp json.Number `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, fmt.Errorf("parsing JWT claims: %w", err)
	}

	expInt, err := claims.Exp.Int64()
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing exp claim: %w", err)
	}

	return time.Unix(expInt, 0), nil
}

// TokenStore persists and retrieves token data keyed by issuer URL.
type TokenStore interface {
	Save(issuerURL string, data *TokenData) error
	Load(issuerURL string) (*TokenData, error)
	Delete(issuerURL string) error
}

// NewTokenStore returns a TokenStore backed by the OS keyring if available,
// falling back to a file-based store otherwise. It returns an error if the
// file fallback is required and the home directory cannot be resolved.
func NewTokenStore() (TokenStore, error) {
	if err := keyring.Set(keyringService, "__probe__", "probe"); err != nil {
		return newFileStore()
	}
	_ = keyring.Delete(keyringService, "__probe__")
	return &keyringStore{}, nil
}

// normalizeIssuer strips trailing slashes from the issuer URL for use as
// a consistent cache key.
func normalizeIssuer(issuerURL string) string {
	return strings.TrimRight(issuerURL, "/")
}

// keyringStore stores tokens in the OS keyring.
type keyringStore struct{}

func (s *keyringStore) Save(issuerURL string, data *TokenData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshalling token data: %w", err)
	}
	return keyring.Set(keyringService, normalizeIssuer(issuerURL), string(b))
}

func (s *keyringStore) Load(issuerURL string) (*TokenData, error) {
	val, err := keyring.Get(keyringService, normalizeIssuer(issuerURL))
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("reading from keyring: %w", err)
	}
	var data TokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("parsing stored token data: %w", err)
	}
	return &data, nil
}

func (s *keyringStore) Delete(issuerURL string) error {
	err := keyring.Delete(keyringService, normalizeIssuer(issuerURL))
	if err == keyring.ErrNotFound {
		return nil
	}
	return err
}

// fileStore stores tokens in a JSON file under ~/.dcm/.
type fileStore struct {
	dir string
}

// userHomeDir is os.UserHomeDir by default; tests may override it.
var userHomeDir = os.UserHomeDir

func newFileStore() (*fileStore, error) {
	home, err := userHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory for token store: %w", err)
	}
	if home == "" {
		return nil, fmt.Errorf("resolving home directory for token store: empty home")
	}
	return &fileStore{dir: filepath.Join(home, ".dcm")}, nil
}

func (s *fileStore) path() string {
	return filepath.Join(s.dir, "tokens.json")
}

func (s *fileStore) readAll() (map[string]*TokenData, error) {
	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*TokenData), nil
		}
		return nil, fmt.Errorf("reading token file: %w", err)
	}
	var store map[string]*TokenData
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parsing token file: %w", err)
	}
	if store == nil {
		store = make(map[string]*TokenData)
	}
	return store, nil
}

func (s *fileStore) writeAll(store map[string]*TokenData) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}
	data, err := json.Marshal(store)
	if err != nil {
		return fmt.Errorf("marshalling token data: %w", err)
	}
	tmpPath := s.path() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path()); err != nil {
		return fmt.Errorf("saving token file: %w", err)
	}
	return nil
}

func (s *fileStore) Save(issuerURL string, td *TokenData) error {
	store, err := s.readAll()
	if err != nil {
		return err
	}
	store[normalizeIssuer(issuerURL)] = td
	return s.writeAll(store)
}

func (s *fileStore) Load(issuerURL string) (*TokenData, error) {
	store, err := s.readAll()
	if err != nil {
		return nil, err
	}
	return store[normalizeIssuer(issuerURL)], nil
}

func (s *fileStore) Delete(issuerURL string) error {
	store, err := s.readAll()
	if err != nil {
		return err
	}
	delete(store, normalizeIssuer(issuerURL))
	return s.writeAll(store)
}

// NewFileStoreWithDir creates a file-based token store at a custom directory,
// used for testing.
func NewFileStoreWithDir(dir string) TokenStore {
	return &fileStore{dir: dir}
}
