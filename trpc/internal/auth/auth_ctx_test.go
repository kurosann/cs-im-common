package auth_test

import (
	"context"
	"testing"

	"vane-common/trpc/common"
	"vane-common/trpc/internal/auth"
)

func TestCtxAuthTokenUsesContextToken(t *testing.T) {
	creds := auth.CtxAuthToken{Fallback: common.InternalToken}
	ctx := context.WithValue(context.Background(), common.Token, "user-access")
	md, err := creds.GetRequestMetadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := md[common.AuthHeaderKey]; got != "Bearer user-access" {
		t.Fatalf("authorization = %q, want Bearer user-access", got)
	}
}

func TestCtxAuthTokenFallsBack(t *testing.T) {
	creds := auth.CtxAuthToken{Fallback: common.InternalToken}
	md, err := creds.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := md[common.AuthHeaderKey]; got != "Bearer "+common.InternalToken {
		t.Fatalf("authorization = %q, want Bearer %s", got, common.InternalToken)
	}
}

func TestCtxAuthTokenMissingErrors(t *testing.T) {
	creds := auth.CtxAuthToken{}
	if _, err := creds.GetRequestMetadata(context.Background()); err == nil {
		t.Fatal("expected error when token and fallback are empty")
	}
}
