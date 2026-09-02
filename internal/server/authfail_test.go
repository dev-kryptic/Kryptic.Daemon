package server

import (
	"errors"
	"testing"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/authstore"
)

func TestIsHardSessionFailure(t *testing.T) {
	if isHardSessionFailure(nil) {
		t.Fatal("nil should not be hard")
	}
	if !isHardSessionFailure(authstore.ErrNotLoggedIn) {
		t.Fatal("missing session is hard")
	}
	if !isHardSessionFailure(&api.APIError{Status: 401, Message: "Invalid refresh token."}) {
		t.Fatal("invalid refresh is hard")
	}
	if !isHardSessionFailure(&api.APIError{Status: 401, Message: "This daemon session has expired by organization policy - run `kryptic login` again."}) {
		t.Fatal("org expiry is hard")
	}
	if isHardSessionFailure(&api.APIError{Status: 401, Message: "unauthorized"}) {
		t.Fatal("plain 401 from /me is not a hard session failure")
	}
	if isHardSessionFailure(errors.New("Post https://daemon.kryptic.dev/api/auth/me: context deadline exceeded")) {
		t.Fatal("timeout is not a hard session failure")
	}
	if isHardSessionFailure(&api.APIError{Status: 503, Message: "unavailable"}) {
		t.Fatal("5xx is not a hard session failure")
	}
}

func TestAuthenticatedWhenSessionPresent(t *testing.T) {
	if !sessionReadsAsSignedIn(true, errors.New("timeout")) {
		t.Fatal("local session plus timeout must still read as signed in")
	}
	if sessionReadsAsSignedIn(true, authstore.ErrNotLoggedIn) {
		t.Fatal("hard failure must read as signed out")
	}
	if sessionReadsAsSignedIn(false, errors.New("timeout")) {
		t.Fatal("no session is signed out")
	}
	if !sessionReadsAsSignedIn(true, nil) {
		t.Fatal("healthy session is signed in")
	}
}
