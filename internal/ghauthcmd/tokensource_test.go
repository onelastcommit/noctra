package ghauthcmd

import (
	"context"
	"errors"
	"testing"

	"github.com/onelastcommit/noctra/internal/ghauth"
	"github.com/onelastcommit/noctra/internal/github"
)

func TestAppTokenSource_MarksNotInstalled(t *testing.T) {
	src := appTokenSource(func(context.Context, string) (string, error) {
		return "", &ghauth.ServiceError{Status: 404, Code: "not_installed", Message: "noctra-agent is not installed"}
	})

	_, err := src(context.Background(), "old/repo")
	if !errors.Is(err, github.ErrAppNotInstalled) {
		t.Fatalf("want ErrAppNotInstalled, got %v", err)
	}
	if !ghauth.IsServiceError(err, "not_installed") {
		t.Fatalf("the service error must stay in the chain, got %v", err)
	}
}

func TestAppTokenSource_PassesOtherResultsThrough(t *testing.T) {
	src := appTokenSource(func(context.Context, string) (string, error) {
		return "", &ghauth.ServiceError{Status: 500, Code: "unexpected_response"}
	})
	if _, err := src(context.Background(), "a/b"); errors.Is(err, github.ErrAppNotInstalled) {
		t.Fatalf("only not_installed is marked, got %v", err)
	}

	ok := appTokenSource(func(context.Context, string) (string, error) { return "tok", nil })
	if token, err := ok(context.Background(), "a/b"); err != nil || token != "tok" {
		t.Fatalf("got %q, %v", token, err)
	}
}
