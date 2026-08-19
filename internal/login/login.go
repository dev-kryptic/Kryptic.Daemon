// Package login implements the browser device flow and sign-out shared by the
// kryptic CLI and the tray apps.
package login

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/server"
)

// Run performs the device flow: a fresh sealed-box key pair is generated for
// this device and its public key registered with the login, `notify` fires
// once with the user code and verification URL (the CLI prints them, the tray
// shows a menu item), the browser is opened, and the flow polls until approval
// or expiry. The private key is stored with the session - it is what lets this
// device open the org-key grant an admin seals to it.
func Run(client *api.Client, notify func(userCode, verificationURL string)) (*api.Me, error) {
	keyPair, err := sealedbox.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	enc := base64.RawURLEncoding
	devicePublicKey := enc.EncodeToString(keyPair.Public)

	hostname, _ := os.Hostname()
	start, err := client.DeviceStart(hostname, runtime.GOOS, server.Version, devicePublicKey)
	if err != nil {
		return nil, err
	}

	if notify != nil {
		notify(start.UserCode, start.VerificationUrl)
	}
	OpenBrowser(start.VerificationUrl)

	deadline := time.Now().Add(time.Duration(start.ExpiresInSeconds) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(start.PollIntervalSeconds) * time.Second)

		tokens, err := client.DevicePoll(start.DeviceCode)
		if err != nil {
			return nil, err
		}
		if tokens == nil {
			continue // still pending
		}

		session := authstore.Session{
			RefreshToken:     tokens.RefreshToken,
			DevicePublicKey:  devicePublicKey,
			DevicePrivateKey: enc.EncodeToString(keyPair.Private),
		}
		if err := authstore.SaveSession(session); err != nil {
			return nil, err
		}
		return client.Me(tokens.AccessToken)
	}
	return nil, fmt.Errorf("the sign-in code expired - try signing in again")
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
	return authstore.Clear()
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
