package ghauth

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"
)

type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type Mode string

const (
	ModeApp   Mode = "app"
	ModeToken Mode = "token"
)

type Installation struct {
	ID      int64  `json:"id"`
	Account string `json:"account"`
}

type Instance struct {
	InstanceID    string         `json:"instance_id"`
	ServiceURL    string         `json:"service_url"`
	GitHubUserID  int64          `json:"github_user_id"`
	GitHubLogin   string         `json:"github_login"`
	AppSlug       string         `json:"app_slug"`
	BotLogin      string         `json:"bot_login"`
	BotUserID     int64          `json:"bot_user_id"`
	BotEmail      string         `json:"bot_email"`
	Installations []Installation `json:"installations"`
	LinkedAt      time.Time      `json:"linked_at"`
}

type Scope string

const (
	ScopeWrite Scope = "write"
	ScopeGit   Scope = "git"
	ScopeRead  Scope = "read"
)

type Bot struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type AppConfig struct {
	AppID      int64  `json:"app_id"`
	ClientID   string `json:"client_id"`
	Slug       string `json:"slug"`
	InstallURL string `json:"install_url"`
	Bot        Bot    `json:"bot"`
}

type GitHubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type LinkResult struct {
	InstanceID    string         `json:"instance_id"`
	GitHubUser    GitHubUser     `json:"github_user"`
	Installations []Installation `json:"installations"`
	App           AppConfig      `json:"app"`
}

type Token struct {
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expires_at"`
	Repository string    `json:"repository"`
	Scope      Scope     `json:"scope"`
}

type ServiceError struct {
	Status     int    `json:"-"`
	Code       string `json:"error"`
	Message    string `json:"message"`
	InstallURL string `json:"install_url"`
}

func (e *ServiceError) Error() string {
	msg := fmt.Sprintf("token service: %s (HTTP %d)", e.Code, e.Status)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if e.InstallURL != "" {
		msg += " Install it at " + e.InstallURL
	}
	return msg
}

type minter interface {
	Token(ctx context.Context, instanceID string, key ed25519.PrivateKey, ownerRepo string, scope Scope) (Token, error)
}
