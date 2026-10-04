package doctor

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func fakeProbe(env map[string]string, outputs map[string]string, files map[string]string) authProbe {
	return authProbe{
		run: func(_ context.Context, name string, args ...string) (string, error) {
			out, ok := outputs[name]
			if !ok {
				return "", errors.New("exit status 1")
			}
			return out, nil
		},
		getenv: func(k string) string { return env[k] },
		readFile: func(path string) ([]byte, error) {
			data, ok := files[path]
			if !ok {
				return nil, os.ErrNotExist
			}
			return []byte(data), nil
		},
		home: "/home/u",
	}
}

func TestDetectAgentAuth(t *testing.T) {
	agySettings := "/home/u/.gemini/antigravity-cli/settings.json"
	cases := []struct {
		name    string
		backend string
		env     map[string]string
		outputs map[string]string
		files   map[string]string
		method  string
		source  string
	}{
		{
			name:    "claude api key wins over login",
			backend: "claude",
			env:     map[string]string{"ANTHROPIC_API_KEY": "sk-ant"},
			outputs: map[string]string{"claude": `{"loggedIn":true,"authMethod":"claude.ai"}`},
			method:  authAPIKey,
			source:  "ANTHROPIC_API_KEY",
		},
		{
			name:    "claude subscription with plan",
			backend: "claude",
			outputs: map[string]string{"claude": `{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty","subscriptionType":"max"}`},
			method:  authSubscription,
			source:  "claude.ai, max plan",
		},
		{
			name:    "claude oauth token login",
			backend: "claude",
			outputs: map[string]string{"claude": `{"loggedIn":true,"authMethod":"oauth_token","apiProvider":"firstParty"}`},
			method:  authSubscription,
			source:  "oauth_token",
		},
		{
			name:    "claude oauth token with plan",
			backend: "claude",
			outputs: map[string]string{"claude": `{"loggedIn":true,"authMethod":"oauth_token","apiProvider":"firstParty","subscriptionType":"pro"}`},
			method:  authSubscription,
			source:  "oauth_token, pro plan",
		},
		{
			name:    "claude unfamiliar method with plan",
			backend: "claude",
			outputs: map[string]string{"claude": `{"loggedIn":true,"authMethod":"something_new","subscriptionType":"max"}`},
			method:  authSubscription,
		},
		{
			name:    "claude console api key login",
			backend: "claude",
			outputs: map[string]string{"claude": `{"loggedIn":true,"authMethod":"api_key","apiProvider":"firstParty"}`},
			method:  authAPIKey,
			source:  "api_key",
		},
		{
			name:    "claude via bedrock",
			backend: "claude",
			outputs: map[string]string{"claude": `{"loggedIn":true,"apiProvider":"bedrock"}`},
			method:  authCloud,
			source:  "bedrock",
		},
		{
			name:    "claude not logged in",
			backend: "claude",
			outputs: map[string]string{"claude": `{"loggedIn":false}`},
			method:  authNotLoggedIn,
		},
		{
			name:    "claude unreadable status",
			backend: "claude",
			method:  authUnknown,
		},
		{
			name:    "codex chatgpt",
			backend: "codex",
			outputs: map[string]string{"codex": "Logged in using ChatGPT\n"},
			method:  authSubscription,
			source:  "ChatGPT",
		},
		{
			name:    "codex api key",
			backend: "codex",
			outputs: map[string]string{"codex": "Logged in using an API key - sk-***\n"},
			method:  authAPIKey,
		},
		{
			name:    "codex env key without login is not guessed",
			backend: "codex",
			env:     map[string]string{"OPENAI_API_KEY": "sk"},
			outputs: map[string]string{"codex": "Not logged in\n"},
			method:  authUnknown,
		},
		{
			name:    "codex not logged in",
			backend: "codex",
			outputs: map[string]string{"codex": "Not logged in\n"},
			method:  authNotLoggedIn,
		},
		{
			name:    "copilot env token",
			backend: "copilot",
			env:     map[string]string{"GH_TOKEN": "gho_x"},
			method:  authSubscription,
			source:  "Copilot plan, token in GH_TOKEN",
		},
		{
			name:    "copilot gh login",
			backend: "copilot",
			method:  authSubscription,
			source:  "Copilot plan, gh login",
		},
		{
			name:    "antigravity api key",
			backend: "antigravity",
			env:     map[string]string{"GEMINI_API_KEY": "k"},
			files:   map[string]string{agySettings: `{"modelProvider":"gemini"}`},
			method:  authAPIKey,
			source:  "GEMINI_API_KEY",
		},
		{
			name:    "antigravity key alone is not enough",
			backend: "antigravity",
			env:     map[string]string{"GEMINI_API_KEY": "k"},
			method:  authUnknown,
		},
		{
			name:    "antigravity provider without key",
			backend: "antigravity",
			files:   map[string]string{agySettings: `{"modelProvider":"gemini"}`},
			method:  authUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fakeProbe(tc.env, tc.outputs, tc.files).detect(context.Background(), tc.backend)
			if got.method != tc.method {
				t.Errorf("method = %q, want %q (source %q)", got.method, tc.method, got.source)
			}
			if tc.source != "" && got.source != tc.source {
				t.Errorf("source = %q, want %q", got.source, tc.source)
			}
		})
	}
}

func TestCheckAgentAuth_SubscriptionAddsDocsNote(t *testing.T) {
	c := checkAgentAuth(fakeProbe(nil, map[string]string{"codex": "Logged in using ChatGPT"}, nil), "codex")
	if !c.ok {
		t.Error("agent auth is informational and must never fail")
	}
	if !strings.Contains(c.note, backendsDocsURL) {
		t.Errorf("subscription login should link the backends docs; note = %q", c.note)
	}
}

func TestCheckAgentAuth_NeverFails(t *testing.T) {
	for _, backend := range []string{"claude", "codex", "copilot", "antigravity"} {
		c := checkAgentAuth(fakeProbe(nil, nil, nil), backend)
		if !c.ok {
			t.Errorf("%s: agent auth must stay informational; detail = %q", backend, c.detail)
		}
	}
	c := checkAgentAuth(fakeProbe(map[string]string{"ANTHROPIC_API_KEY": "k"}, nil, nil), "claude")
	if c.note != "" {
		t.Errorf("API key auth should not add the subscription note; note = %q", c.note)
	}
}
