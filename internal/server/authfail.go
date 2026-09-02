package server

import (
	"errors"
	"strings"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/authstore"
)

func isUnauthorized(err error) bool {
	var apiError *api.APIError
	return errors.As(err, &apiError) && apiError.Status == httpStatusUnauthorized
}

const httpStatusUnauthorized = 401

func isInvalidRefresh(err error) bool {
	if !isUnauthorized(err) {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "invalid refresh")
}

// isHardSessionFailure is a real logout: no local session, or the platform
// rejected the refresh token. Network blips and /me 401s are not this.
func isHardSessionFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, authstore.ErrNotLoggedIn) {
		return true
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "invalid refresh") {
		return true
	}
	if strings.Contains(text, "expired by organization") {
		return true
	}
	if strings.Contains(text, "deactivated") {
		return true
	}
	return false
}

// sessionReadsAsSignedIn is the tray/CLI rule: a stored session stays signed
// in through transient platform errors. Only a missing or rejected session
// shows as logged out.
func sessionReadsAsSignedIn(hasSession bool, err error) bool {
	if !hasSession {
		return false
	}
	return !isHardSessionFailure(err)
}
