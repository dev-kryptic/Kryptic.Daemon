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
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/server"
)

// Run performs the device flow: a fresh sealed-box key pair is generated for
// this device and its public key registered with the login, `notify` fires
// once with the user code and verification URL (the CLI prints them, the tray
// shows a menu item), the browser is opened, and the flow polls until approval
// or expiry. The private key is stored with the session - it is what lets this
// device open the org-key grant an admin seals to it.
func Run(client *api.Client, notify func(userCode, verificationURL string)) (*api.Me, error) {
	return RunContext(context.Background(), client, notify)
}

// RunContext is Run with cancellation (the tray Cancel Sign-In action).
func RunContext(ctx context.Context, client *api.Client, notify func(userCode, verificationURL string)) (*api.Me, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	keyPair, err := sealedbox.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	enc := base64.RawURLEncoding
	devicePublicKey := enc.EncodeToString(keyPair.Public)

	hostname, _ := os.Hostname()
	start, err := client.DeviceStart(hostname, runtime.GOOS, server.Version, devicePublicKey)
	if err != nil {
		applog.Error("cli", "auth.login.start", err, "result=error")
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
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

		tokens, err := client.DevicePoll(start.DeviceCode)
		if err != nil {
			applog.Error("cli", "auth.login.poll", err, "result=error")
			return nil, err
		}
		if tokens == nil {
			continue
		}

		session := authstore.Session{
			RefreshToken:     tokens.RefreshToken,
			DevicePublicKey:  devicePublicKey,
			DevicePrivateKey: enc.EncodeToString(keyPair.Private),
		}
		if err := authstore.SaveSession(session); err != nil {
			applog.Error("cli", "auth.login.save", err)
			return nil, err
		}
		applog.Event("cli", "auth.login.ok")
		return client.Me(tokens.AccessToken)
	}
}

// Logout revokes the server-side session (best effort) and clears the stored
// session - including the device private key, so this device can no longer
// decrypt even if ciphertext was captured.
func Logout(client *api.Client) error {
	session, err := authstore.LoadSession()
	if err == nil {
		if tokens, refreshErr := client.Refresh(session.RefreshToken); refreshErr == nil {
			_ = client.Logout(tokens.AccessToken)
		}
	}
	err = authstore.Clear()
	applog.Event("cli", "auth.logout")

	// A running daemon keeps its access token and decrypted bundles in memory
	// for up to 15 minutes. Tell it to drop them so `kryptic status` and the
	// language packages see the sign-out immediately (best effort - the daemon
	// may not be running).
	_, _ = ipc.Request(map[string]any{"type": "reset-auth"})

	return err
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
