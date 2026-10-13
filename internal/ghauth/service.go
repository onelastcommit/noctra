package ghauth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultServiceURL = "https://auth.getnoctra.dev"

func ParseScope(s string) (Scope, error) {
	switch Scope(s) {
	case ScopeWrite, ScopeGit, ScopeRead:
		return Scope(s), nil
	}
	return "", fmt.Errorf("unknown scope %q (want write, git or read)", s)
}

func IsServiceError(err error, code string) bool {
	var se *ServiceError
	return errors.As(err, &se) && se.Code == code
}

type Service struct {
	BaseURL string
	HTTP    *http.Client
	Now     func() time.Time
}

func NewService(baseURL string) *Service {
	return &Service{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 20 * time.Second},
		Now:     time.Now,
	}
}

func (s *Service) Config(ctx context.Context) (AppConfig, error) {
	var cfg AppConfig
	err := s.do(ctx, http.MethodGet, "/config", nil, nil, &cfg)
	return cfg, err
}

func (s *Service) Health(ctx context.Context) error {
	return s.do(ctx, http.MethodGet, "/healthz", nil, nil, nil)
}

func (s *Service) Link(ctx context.Context, githubToken string, key ed25519.PrivateKey) (LinkResult, error) {
	body, err := json.Marshal(map[string]string{
		"github_token": githubToken,
		"public_key":   EncodePublicKey(key.Public().(ed25519.PublicKey)),
	})
	if err != nil {
		return LinkResult{}, err
	}
	var out LinkResult
	headers := SignedHeaders(key, "", http.MethodPost, "/link", body, s.Now(), NewNonce())
	err = s.do(ctx, http.MethodPost, "/link", body, headers, &out)
	return out, err
}

func (s *Service) Token(ctx context.Context, instanceID string, key ed25519.PrivateKey, ownerRepo string, scope Scope) (Token, error) {
	body, err := json.Marshal(map[string]string{"repository": ownerRepo, "scope": string(scope)})
	if err != nil {
		return Token{}, err
	}
	var out Token
	headers := SignedHeaders(key, instanceID, http.MethodPost, "/token", body, s.Now(), NewNonce())
	err = s.do(ctx, http.MethodPost, "/token", body, headers, &out)
	if err == nil && out.Token == "" {
		err = errors.New("token service returned an empty token")
	}
	return out, err
}

func (s *Service) Unlink(ctx context.Context, instanceID string, key ed25519.PrivateKey) error {
	body := []byte("{}")
	headers := SignedHeaders(key, instanceID, http.MethodPost, "/unlink", body, s.Now(), NewNonce())
	return s.do(ctx, http.MethodPost, "/unlink", body, headers, nil)
}

func (s *Service) do(ctx context.Context, method, path string, body []byte, headers map[string]string, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "noctra")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("token service %s: %w", path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("token service %s: %w", path, err)
	}
	if resp.StatusCode >= 300 {
		se := &ServiceError{Status: resp.StatusCode}
		if json.Unmarshal(data, se) != nil || se.Code == "" {
			se.Code = "unexpected_response"
			se.Message = http.StatusText(resp.StatusCode)
		}
		return se
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("token service %s: decode response: %w", path, err)
	}
	return nil
}
