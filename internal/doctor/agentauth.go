package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/onelastcommit/noctra/internal/authcheck"
)

const backendsDocsURL = "https://getnoctra.dev/docs#backends"

const (
	authSubscription = "subscription login"
	authAPIKey       = "API key"
	authCloud        = "cloud provider"
	authNotLoggedIn  = "not logged in"
	authUnknown      = "unknown"
)

type agentAuth struct {
	method string
	source string
}

type authProbe struct {
	run      authcheck.Exec
	getenv   func(string) string
	readFile func(string) ([]byte, error)
	home     string
}

func defaultAuthProbe() authProbe {
	home, _ := os.UserHomeDir()
	return authProbe{run: authcheck.DefaultExec, getenv: os.Getenv, readFile: os.ReadFile, home: home}
}

func checkAgentAuth(p authProbe, backend string) check {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a := p.detect(ctx, backend)
	c := check{name: "agent auth", ok: true, detail: fmt.Sprintf("%s (%s)", a.method, a.source)}
	if a.method == authSubscription {
		c.note = "Subscription usage counts against your plan's limits and terms: " + backendsDocsURL
	}
	return c
}

func (p authProbe) detect(ctx context.Context, backend string) agentAuth {
	switch backend {
	case "claude":
		return p.claude(ctx)
	case "codex":
		return p.codex(ctx)
	case "copilot":
		return p.copilot()
	case "antigravity":
		return p.antigravity()
	default:
		return agentAuth{authUnknown, "unrecognised backend " + backend}
	}
}

func (p authProbe) claude(ctx context.Context) agentAuth {
	if p.getenv("ANTHROPIC_API_KEY") != "" {
		return agentAuth{authAPIKey, "ANTHROPIC_API_KEY"}
	}
	out, _ := p.run(ctx, "claude", "auth", "status", "--json")
	var status struct {
		LoggedIn         bool   `json:"loggedIn"`
		AuthMethod       string `json:"authMethod"`
		APIProvider      string `json:"apiProvider"`
		SubscriptionType string `json:"subscriptionType"`
	}
	if err := json.Unmarshal([]byte(authcheck.JSONObject(out)), &status); err != nil {
		return agentAuth{authUnknown, "could not read `claude auth status`"}
	}
	switch {
	case !status.LoggedIn:
		return agentAuth{authNotLoggedIn, "claude auth status"}
	case status.APIProvider != "" && status.APIProvider != "firstParty":
		return agentAuth{authCloud, status.APIProvider}
	case status.AuthMethod == "claude.ai" || status.AuthMethod == "oauth_token" || status.SubscriptionType != "":
		if status.SubscriptionType != "" {
			return agentAuth{authSubscription, status.AuthMethod + ", " + status.SubscriptionType + " plan"}
		}
		return agentAuth{authSubscription, status.AuthMethod}
	case strings.Contains(strings.ToLower(status.AuthMethod), "api"):
		return agentAuth{authAPIKey, status.AuthMethod}
	default:
		return agentAuth{authUnknown, "authMethod " + status.AuthMethod}
	}
}

func (p authProbe) codex(ctx context.Context) agentAuth {
	out, _ := p.run(ctx, "codex", "login", "status")
	lower := strings.ToLower(out)
	switch {
	case strings.Contains(lower, "chatgpt"):
		return agentAuth{authSubscription, "ChatGPT"}
	case strings.Contains(lower, "api key"):
		return agentAuth{authAPIKey, "codex login status"}
	case p.getenv("OPENAI_API_KEY") != "":
		return agentAuth{authUnknown, "OPENAI_API_KEY is set but `codex login status` reports no login"}
	case strings.Contains(lower, "not logged in"):
		return agentAuth{authNotLoggedIn, "codex login status"}
	default:
		return agentAuth{authUnknown, "could not read `codex login status`"}
	}
}

func (p authProbe) copilot() agentAuth {
	for _, key := range []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if p.getenv(key) != "" {
			return agentAuth{authSubscription, "Copilot plan, token in " + key}
		}
	}
	return agentAuth{authSubscription, "Copilot plan, gh login"}
}

func (p authProbe) antigravity() agentAuth {
	var settings struct {
		ModelProvider string `json:"modelProvider"`
	}
	if data, err := p.readFile(filepath.Join(p.home, ".gemini", "antigravity-cli", "settings.json")); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	if settings.ModelProvider == "gemini" && p.getenv("GEMINI_API_KEY") != "" {
		return agentAuth{authAPIKey, "GEMINI_API_KEY"}
	}
	return agentAuth{authUnknown, "agy has no non-interactive auth status"}
}
