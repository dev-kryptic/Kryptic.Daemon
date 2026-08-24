// The kryptic CLI. `kryptic start` runs the daemon in the foreground
// (launchd/systemd/the service manager keep it alive); the other commands talk
// to the platform or to the running daemon's socket.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/authstore"
	"github.com/dev-kryptic/daemon/internal/config"
	"github.com/dev-kryptic/daemon/internal/ipc"
	"github.com/dev-kryptic/daemon/internal/login"
	"github.com/dev-kryptic/daemon/internal/pidfile"
	"github.com/dev-kryptic/daemon/internal/scan"
	"github.com/dev-kryptic/daemon/internal/server"
	"github.com/dev-kryptic/daemon/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}

	client := api.NewClient()

	var err error
	switch os.Args[1] {
	case "start":
		err = runStart(client)
	case "stop":
		err = runStop()
	case "update":
		err = runUpdate()
	case "config":
		err = runConfig()
	case "scan":
		err = runScan()
	case "login":
		err = runLogin(client)
	case "logout":
		err = runLogout(client)
	case "status":
		err = status()
	case "whoami":
		err = whoami(client)
	case "secrets":
		err = secrets()
	case "ci":
		err = ci()
	case "flush":
		err = flush()
	case "version":
		fmt.Println("kryptic", server.Version)
	default:
		usage()
	}

	if err != nil {
		if errors.Is(err, update.ErrAvailable) {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "kryptic:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`kryptic - the Kryptic daemon and CLI

  kryptic start                 run the daemon (foreground; managed by launchd/systemd)
  kryptic stop                  stop the running daemon
  kryptic login                 sign in via your browser (device flow)
  kryptic logout                revoke this device's session
  kryptic status                daemon + session status
  kryptic whoami                the signed-in user and organization
  kryptic secrets list          projects and environments you can pull
  kryptic secrets get KEY --project proj_x --env development
  kryptic secrets export --project proj_x --env development   print a dotenv (decrypted locally)
  kryptic flush                 clear the daemon's secrets cache (refetch on next request)
  kryptic ci export --project proj_x --env production   pipeline secrets, decrypted locally
  kryptic scan [PATH]           scan files for leaked secrets (also: --staged for the git index)
  kryptic update                update kryptic to the latest release
  kryptic update --check        report whether a newer release exists (exit 2 if so)
  kryptic update --installer    download the signed installer and open it
  kryptic config                show the Daemon BFF URL
  kryptic config set-api URL    save the server URL (signs you out if it changes)
  kryptic config reset-api      return to https://daemon.kryptic.dev
  kryptic version`)
}

// ---------- lifecycle ----------

// runStart records the pid, handles SIGINT/SIGTERM for a clean exit, and runs
// the socket server in the foreground.
func runStart(client *api.Client) error {
	if pid, err := pidfile.Read(); err == nil && pidfile.Alive(pid) && pid != os.Getpid() {
		return fmt.Errorf("daemon already running (pid %d) - `kryptic stop` first", pid)
	}

	if err := pidfile.Write(); err != nil {
		return err
	}
	defer pidfile.Remove()

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interrupts
		pidfile.Remove()
		os.Exit(0)
	}()

	return server.New(client).Run()
}

func runStop() error {
	pid, err := pidfile.Read()
	if err != nil {
		fmt.Println("daemon: not running")
		return nil
	}
	if !pidfile.Alive(pid) {
		pidfile.Clear() // crashed daemon left a stale file
		fmt.Println("daemon: not running (cleaned up a stale pidfile)")
		return nil
	}

	if err := pidfile.Terminate(pid); err != nil {
		return fmt.Errorf("could not stop daemon (pid %d): %w", pid, err)
	}

	for waited := 0; waited < 50; waited++ {
		if !pidfile.Alive(pid) {
			pidfile.Clear()
			fmt.Println("daemon stopped.")
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon (pid %d) did not exit within 5s", pid)
}

// ---------- scan ----------

func runScan() error {
	config, err := scan.Load()
	if err != nil {
		return err
	}

	args := os.Args[2:]
	staged := false
	root := "."
	for _, arg := range args {
		if arg == "--staged" {
			staged = true
		} else {
			root = arg
		}
	}

	var findings []scan.Finding
	if staged {
		diff, err := stagedDiff()
		if err != nil {
			return err
		}
		findings = config.ScanContent("(staged diff)", diff)
	} else {
		findings, err = config.ScanPath(root)
		if err != nil {
			return err
		}
	}

	if scan.Report(findings) {
		os.Exit(1) // CI-friendly: findings fail the build
	}
	return nil
}

// ---------- auth ----------

func runLogin(client *api.Client) error {
	me, err := login.Run(client, func(userCode, verificationURL string) {
		fmt.Printf("Confirm this code in your browser: %s\n%s\n", userCode, verificationURL)
	})
	if err != nil {
		return err
	}
	fmt.Printf("Signed in as %s (%s). The daemon can now serve secrets.\n", me.Email, me.Organization)
	return nil
}

func runLogout(client *api.Client) error {
	if err := login.Logout(client); err != nil {
		return err
	}
	fmt.Println("Signed out.")
	return nil
}

// platformAccessToken exchanges the stored refresh token for an access token,
// persisting the rotated refresh token (and keeping the device keys) in place.
func platformAccessToken(client *api.Client) (string, error) {
	session, err := authstore.LoadSession()
	if err != nil {
		return "", err
	}
	tokens, err := client.Refresh(session.RefreshToken)
	if err != nil {
		return "", err
	}
	session.RefreshToken = tokens.RefreshToken
	if err := authstore.SaveSession(session); err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

// whoami asks the platform directly (works even when the daemon isn't running).
func whoami(client *api.Client) error {
	accessToken, err := platformAccessToken(client)
	if err != nil {
		return err
	}
	me, err := client.Me(accessToken)
	if err != nil {
		return err
	}
	fmt.Printf("%s (%s) - organization: %s\n", me.Email, me.DisplayName, me.Organization)
	return nil
}

// ---------- socket-backed commands ----------

func status() error {
	apiURL, source := config.API()
	response, err := ipc.Request(map[string]any{"type": "status"})
	if err != nil {
		fmt.Println("daemon: not running")
		fmt.Printf("api: %s (%s)\n", apiURL, source)
		return nil
	}
	if response["authenticated"] == true {
		fmt.Printf("daemon: online (v%v) - signed in as %v @ %v\n",
			response["daemonVersion"], response["email"], response["organization"])
	} else {
		fmt.Printf("daemon: online (v%v) - not signed in (run `kryptic login`)\n", response["daemonVersion"])
	}
	if reported, ok := response["apiUrl"].(string); ok && reported != "" {
		apiURL = reported
	}
	fmt.Printf("api: %s (%s)\n", apiURL, source)
	return nil
}

func runConfig() error {
	args := os.Args[2:]
	if len(args) == 0 {
		url, source := config.API()
		fmt.Printf("api: %s (%s)\n", url, source)
		if config.EnvOverrides() {
			fmt.Println("KRYPTIC_API is set and overrides the saved URL.")
		}
		return nil
	}
	switch args[0] {
	case "set-api":
		if len(args) < 2 {
			return fmt.Errorf("usage: kryptic config set-api URL")
		}
		return applyAPIChange(func() error { return config.SetAPI(args[1]) })
	case "reset-api":
		return applyAPIChange(config.ResetAPI)
	default:
		return fmt.Errorf("usage: kryptic config [set-api URL|reset-api]")
	}
}

func applyAPIChange(write func() error) error {
	previous, _ := config.API()
	previousClient := api.NewClientFor(previous)
	if err := write(); err != nil {
		return err
	}
	next, source := config.API()
	fmt.Printf("api: %s (%s)\n", next, source)
	if config.EnvOverrides() {
		fmt.Println("KRYPTIC_API is set and still overrides the saved URL.")
		return nil
	}
	if previous == next {
		return nil
	}
	hadSession := false
	if _, err := authstore.LoadSession(); err == nil {
		hadSession = true
	}
	_ = login.Logout(previousClient)
	if hadSession {
		fmt.Println("signed out of the previous server. Run `kryptic login` against the new one.")
	} else {
		fmt.Println("run `kryptic login` against the new server.")
	}
	if _, err := ipc.Request(map[string]any{"type": "status"}); err == nil {
		update.RestartDaemon()
	}
	return nil
}

func runUpdate() error {
	checkOnly := false
	useInstaller := false
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--check":
			checkOnly = true
		case "--installer":
			useInstaller = true
		default:
			return fmt.Errorf("usage: kryptic update [--check|--installer]")
		}
	}
	if checkOnly {
		return update.PrintCheck(server.Version)
	}
	if useInstaller {
		return update.RunInstaller(server.Version)
	}
	return update.Run(server.Version)
}

func flush() error {
	response, err := ipc.Request(map[string]any{"type": "flush"})
	if err != nil {
		return err
	}
	if response["ok"] != true {
		return fmt.Errorf("%v", response["message"])
	}
	fmt.Printf("secrets cache cleared (%v bundle(s) dropped)\n", response["cleared"])
	return nil
}

func secrets() error {
	if len(os.Args) < 3 {
		usage()
		return nil
	}

	switch os.Args[2] {
	case "list":
		return secretsList()
	case "get":
		return secretsGet()
	case "export":
		return secretsExport()
	default:
		usage()
		return nil
	}
}

// secretsList goes through the platform (the daemon socket serves per-project bundles).
func secretsList() error {
	client := api.NewClient()
	accessToken, err := platformAccessToken(client)
	if err != nil {
		return err
	}
	projects, err := client.Projects(accessToken)
	if err != nil {
		return err
	}
	for _, project := range projects {
		fmt.Printf("%s  %s  %v\n", project.PublicId, project.Name, project.Environments)
	}
	return nil
}

// stagedDiff returns the added lines of the git index - what a pre-commit
// hook wants scanned.
func stagedDiff() (string, error) {
	output, err := exec.Command("git", "diff", "--cached", "--unified=0").Output()
	if err != nil {
		return "", fmt.Errorf("git diff --cached failed - is this a git repository?")
	}

	var added []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added = append(added, strings.TrimPrefix(line, "+"))
		}
	}
	return strings.Join(added, "\n"), nil
}

func secretsGet() error {
	key, projectId, environment := "", "", "development"
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			i++
			projectId = args[i]
		case "--env":
			i++
			environment = args[i]
		default:
			key = args[i]
		}
	}
	if key == "" || projectId == "" {
		return fmt.Errorf("usage: kryptic secrets get KEY --project proj_x [--env development]")
	}

	response, err := ipc.Request(map[string]any{"type": "secrets", "projectId": projectId, "environment": environment})
	if err != nil {
		return err
	}
	if response["ok"] != true {
		return fmt.Errorf("%v", response["message"])
	}
	entries, _ := response["secrets"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if entry["key"] == key {
			fmt.Println(entry["value"])
			return nil
		}
	}
	return fmt.Errorf("no secret named %s in %s/%s", key, projectId, environment)
}

// secretsExport prints a dotenv rendering of an environment. Decryption already
// happened inside the daemon on this machine - the platform only ever saw
// ciphertext. Quoting matches the management client's formatDotEnv.
func secretsExport() error {
	projectId, environment := "", "development"
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			i++
			projectId = args[i]
		case "--env":
			i++
			environment = args[i]
		}
	}
	if projectId == "" {
		return fmt.Errorf("usage: kryptic secrets export --project proj_x [--env development]")
	}

	response, err := ipc.Request(map[string]any{"type": "secrets", "projectId": projectId, "environment": environment})
	if err != nil {
		return err
	}
	if response["ok"] != true {
		return fmt.Errorf("%v", response["message"])
	}

	entries, _ := response["secrets"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		key, _ := entry["key"].(string)
		value, _ := entry["value"].(string)
		fmt.Println(dotenvLine(key, value))
	}
	return nil
}

// dotenvLine renders one KEY=value line, quoting when the value contains
// whitespace or shell-significant characters.
func dotenvLine(key, value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\r\n#\"'`$\\") {
		return key + "=" + value
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return key + `="` + escaped + `"`
}
