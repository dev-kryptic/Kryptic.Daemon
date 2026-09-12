// Package login implements the browser device flow and sign-out shared by the
// kryptic CLI and the tray apps.
package login

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/possession"
	"github.com/dev-kryptic/daemon/internal/server"
)

// Run performs the device flow for the active profile. Existing keys on that
// profile are reused so an admin grant can outlive one refresh-token session.
func Run(client *api.Client, notify func(userCode, verificationURL string)) (*api.Me, error) {
	return RunContext(context.Background(), client, notify, false)
}

// RunAdd is Run for a second account. It generates a new key pair so personal
// and work grants stay isolated.
func RunAdd(client *api.Client, notify func(userCode, verificationURL string)) (*api.Me, error) {
	return RunContext(context.Background(), client, notify, true)
}

// RunContext is Run with cancellation (the tray Cancel Sign-In action).
func RunContext(ctx context.Context, client *api.Client, notify func(userCode, verificationURL string), add bool) (*api.Me, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	targetAPI := strings.TrimSpace(client.BaseURL)
	if targetAPI == "" {
		targetAPI = authstore.ResolvedAPI()
	}

	enc := base64.RawURLEncoding
	var publicKey, privateKey string
	if !add {
		if existing, err := authstore.LoadSession(); err == nil && authstore.HasDeviceKeys(existing) {
			if existing.API() == "" || existing.API() == authstore.CanonicalAPI(targetAPI) {
				publicKey = existing.DevicePublicKey
				privateKey = existing.DevicePrivateKey
			}
		}
	}
	if publicKey == "" {
		keyPair, err := sealedbox.GenerateKeyPair()
		if err != nil {
			return nil, err
		}
		publicKey = enc.EncodeToString(keyPair.Public)
		privateKey = enc.EncodeToString(keyPair.Private)
	}

	hostname, _ := os.Hostname()
	start, err := client.DeviceStart(hostname, runtime.GOOS, server.Version, publicKey)
	if err != nil {
		applog.Error("cli", "auth.login.start", err, "result=error")
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	signature := ""
	if start.Challenge != "" {
		signature, err = possession.SignChallenge(privateKey, start.Challenge)
		if err != nil {
			applog.Error("cli", "auth.login.sign", err, "result=error")
			return nil, err
		}
	}

	if notify != nil {
		notify(start.UserCode, start.VerificationUrl)
	}
	applog.Event("cli", "auth.login.start")
	OpenBrowser(start.VerificationUrl)

	deadline := time.Now().Add(time.Duration(start.ExpiresInSeconds) * time.Second)
	interval := time.Duration(start.PollIntervalSeconds) * time.Second
	for {
		if err := ctx.Err(); err != nil {
			applog.Event("cli", "auth.login.cancel")
			return nil, err
		}
		if !time.Now().Before(deadline) {
			applog.Event("cli", "auth.login.expired")
			return nil, fmt.Errorf("the sign-in code expired - try signing in again")
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}

		tokens, err := client.DevicePoll(start.DeviceCode, signature)
		if err != nil {
			applog.Error("cli", "auth.login.poll", err, "result=error")
			return nil, err
		}
		if tokens == nil {
			continue
		}

		me, err := client.Me(tokens.AccessToken)
		if err != nil {
			applog.Error("cli", "auth.login.me", err, "result=error")
			return nil, err
		}

		if !add {
			if current, loadErr := authstore.LoadSession(); loadErr == nil && current.Email != "" {
				same := strings.EqualFold(current.Email, me.Email) &&
					current.Organization == me.Organization &&
					current.API() == authstore.CanonicalAPI(targetAPI)
				if !same {
					return nil, fmt.Errorf(
						"browser signed in as %s (%s) on %s, which is not the active profile. Use `kryptic login --add --api %s`",
						me.Email, me.Organization, authstore.CanonicalAPI(targetAPI), authstore.CanonicalAPI(targetAPI),
					)
				}
			}
		}

		session := authstore.Session{
			RefreshToken:     tokens.RefreshToken,
			DevicePublicKey:  publicKey,
			DevicePrivateKey: privateKey,
			Email:            me.Email,
			Organization:     me.Organization,
			Config:           authstore.ProfileConfig{API: authstore.CanonicalAPI(targetAPI)},
		}
		if err := authstore.SaveLoggedIn(session, add); err != nil {
			applog.Error("cli", "auth.login.save", err)
			return nil, err
		}
		_, _ = ipc.Request(map[string]any{"type": "reset-auth"})
		applog.Event("cli", "auth.login.ok")
		return me, nil
	}
}

// Logout revokes the active profile's server-side session and drops the
// in-memory org key. Device keys stay so the next login can reuse the grant.
// Other profiles stay signed in.
func Logout(client *api.Client) error {
	session, err := authstore.LoadSession()
	if err == nil && session.RefreshToken != "" {
		if tokens, refreshErr := client.Refresh(session.RefreshToken); refreshErr == nil {
			_ = client.Logout(tokens.AccessToken)
		}
	}
	err = authstore.ClearRefresh()
	applog.Event("cli", "auth.logout")
	_, _ = ipc.Request(map[string]any{"type": "reset-auth"})
	return err
}

// LogoutAll signs every profile out. Each profile talks to its own server.
func LogoutAll(_ *api.Client) error {
	store, err := authstore.LoadStore()
	if err == nil {
		for _, profile := range store.Profiles {
			revokeProfileSession(profile)
		}
	}
	err = authstore.ClearAllRefresh()
	applog.Event("cli", "auth.logout_all")
	_, _ = ipc.Request(map[string]any{"type": "reset-auth"})
	return err
}

// Switch activates another saved profile and drops in-memory secrets.
func Switch(idOrEmail string) error {
	if err := authstore.Switch(idOrEmail); err != nil {
		return err
	}
	applog.Event("cli", "auth.profile.switch")
	_, _ = ipc.Request(map[string]any{"type": "reset-auth"})
	return nil
}

// ResetDevice deletes durable keys for the active profile and, if still
// signed in, revokes the device server-side.
func ResetDevice(client *api.Client) error {
	session, err := authstore.LoadSession()
	if err == nil && session.RefreshToken != "" {
		if tokens, refreshErr := client.Refresh(session.RefreshToken); refreshErr == nil {
			_ = client.RevokeDevice(tokens.AccessToken)
		}
	}
	err = authstore.RemoveActive()
	applog.Event("cli", "auth.reset_device")
	_, _ = ipc.Request(map[string]any{"type": "reset-auth"})
	return err
}

// ResetAllDevices wipes every profile on this install.
func ResetAllDevices(_ *api.Client) error {
	store, err := authstore.LoadStore()
	if err == nil {
		for _, profile := range store.Profiles {
			revokeProfileDevice(profile)
		}
	}
	err = authstore.Clear()
	applog.Event("cli", "auth.reset_device_all")
	_, _ = ipc.Request(map[string]any{"type": "reset-auth"})
	return err
}

// Delete removes one profile: revoke its session and device on that profile's
// server, then drop its keys from this install.
func Delete(idOrEmail string) error {
	store, err := authstore.LoadStore()
	if err != nil {
		return err
	}
	profile, ok := authstore.FindProfile(store, idOrEmail)
	if !ok {
		return authstore.ErrUnknownProfile
	}
	revokeProfileDevice(profile)
	if err := authstore.Remove(profile.ID); err != nil {
		return err
	}
	applog.Event("cli", "auth.profile.delete")
	_, _ = ipc.Request(map[string]any{"type": "reset-auth"})
	return nil
}

// SetActiveAPI updates the active profile's Daemon BFF and signs that
// profile out of the previous host. Other profiles keep their URL and session.
func SetActiveAPI(raw string) error {
	previous := authstore.ResolvedAPI()
	session, _ := authstore.LoadSession()
	if err := authstore.SetActiveAPI(raw); err != nil {
		return err
	}
	next := authstore.ResolvedAPI()
	if previous != next && session.RefreshToken != "" {
		client := api.NewClientFor(previous)
		if tokens, refreshErr := client.Refresh(session.RefreshToken); refreshErr == nil {
			_ = client.Logout(tokens.AccessToken)
		}
	}
	applog.Event("cli", "config.api")
	_, _ = ipc.Request(map[string]any{"type": "reset-auth"})
	return nil
}

func revokeProfileSession(profile authstore.Profile) {
	if profile.RefreshToken == "" {
		return
	}
	client := api.NewClientFor(profile.API())
	if tokens, err := client.Refresh(profile.RefreshToken); err == nil {
		_ = client.Logout(tokens.AccessToken)
	}
}

func revokeProfileDevice(profile authstore.Profile) {
	if profile.RefreshToken == "" {
		return
	}
	client := api.NewClientFor(profile.API())
	if tokens, err := client.Refresh(profile.RefreshToken); err == nil {
		_ = client.RevokeDevice(tokens.AccessToken)
		_ = client.Logout(tokens.AccessToken)
	}
}

func OpenBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", url).Start()
	case "linux":
		_ = exec.Command("xdg-open", url).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}
