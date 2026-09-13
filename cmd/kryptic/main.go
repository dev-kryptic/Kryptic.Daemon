// The kryptic CLI. `kryptic start` runs the daemon in the foreground
// (launchd/systemd/the service manager keep it alive); the other commands talk
// to the platform or to the running daemon's socket.
//
//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --arch amd64 --in winres/winres.json --out rsrc
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
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
	case "profile":
		err = runProfile()
	case "reset-device":
		err = runResetDevice(client)
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
	case "logs":
		err = runLogs()
	case "panel":
		err = runPanel()
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
  kryptic login --add           sign in another account without leaving the current one
  kryptic login --add --api URL sign in against a different Daemon BFF (self-host vs cloud)
  kryptic logout                revoke the active profile's session (asks to confirm; --yes skips)
  kryptic profile               list saved accounts on this install
  kryptic profile switch ID     switch the active account (email or profile id)
  kryptic profile delete ID     remove an account from this install (revokes that profile)
  kryptic reset-device          delete the active profile's keys (asks to confirm; --yes skips)
  kryptic reset-device --all    wipe every profile on this install
  kryptic status                daemon + session status
  kryptic whoami                the signed-in user and organization
  kryptic secrets list          projects and environments you can pull
  kryptic secrets get KEY --project proj_x --env development
  kryptic secrets export --project proj_x --env development   print a dotenv (decrypted locally)
  kryptic flush                 clear the daemon's secrets cache (refetch on next request)
  kryptic logs                  print the diagnostics log path (send this file to support)
  kryptic logs --reveal         open the diagnostics log in the file manager
  kryptic ci export --project proj_x --env production   pipeline secrets, decrypted locally
  kryptic scan [PATH]           scan files for leaked secrets (--staged, --export [FILE|DIR])
  kryptic update                update kryptic to the latest release
  kryptic update --check        report whether a newer release exists (exit 2 if so)
  kryptic update --installer    download the signed installer and open it (macOS/Windows)
  kryptic config                show the Daemon BFF URL
  kryptic config set-api URL    save the active profile's server URL (that profile only)
  kryptic config reset-api      return to https://daemon.kryptic.dev
  kryptic panel                 open Open Kryptic (Windows/Linux; on macOS use the menu)
  kryptic version`)
}

// ---------- lifecycle ----------

// runStart records the pid, handles SIGINT/SIGTERM for a clean exit, and runs
// the socket server in the foreground.
func runStart(client *api.Client) error {
	if err := pidfile.RefuseRoot("start"); err != nil {
		return err
	}

	if pid, err := pidfile.Read(); err == nil && pidfile.Alive(pid) && pid != os.Getpid() {
		return fmt.Errorf("daemon already running (pid %d) - `kryptic stop` first", pid)
	} else if errors.Is(err, pidfile.ErrUnreadable) {
		return fmt.Errorf("%w. Try: sudo kryptic stop", err)
	}

	if err := pidfile.Write(); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("cannot write pidfile (a daemon may have been started with sudo): %w. Try: sudo kryptic stop", err)
		}
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
	switch {
	case errors.Is(err, pidfile.ErrNotRunning):
		fmt.Println("daemon: not running")
		return nil
	case errors.Is(err, pidfile.ErrUnreadable):
		return fmt.Errorf("%w. Try: sudo kryptic stop", err)
	case err != nil:
		return err
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
			applog.Event("cli", "daemon.stop")
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

	opts, err := scan.ParseArgs(os.Args[2:])
	if err != nil {
		return err
	}

	progress := scan.TerminalProgress()
	if opts.Progress {
		progress = scan.LineProgress(os.Stderr)
	}
	var findings []scan.Finding
	files := 0
	target := opts.Root
	if opts.Staged {
		diff, err := stagedDiff()
		if err != nil {
			return err
		}
		target = "(staged git index)"
		findings = config.ScanContentWithProgress("(staged diff)", diff, progress)
		files = 1
	} else {
		result, err := config.ScanPathWithProgress(opts.Root, progress)
		if err != nil {
			return err
		}
		findings = result.Findings
		files = result.Files
	}

	found := scan.Report(findings)

	if opts.Export {
		path, err := scan.ResolveExportPath(opts.ExportPath)
		if err != nil {
			return err
		}
		meta := scan.ExportMeta{
			Target:    target,
			Staged:    opts.Staged,
			Files:     files,
			Rules:     len(config.Rules),
			Generated: time.Now(),
			Version:   server.Version,
		}
		if err := scan.WriteReport(path, findings, meta); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", path)
	}

	if found {
		os.Exit(1) // CI-friendly: findings fail the build
	}
	return nil
}

// ---------- auth ----------

func runLogin(client *api.Client) error {
	add := hasArg("--add")
	target := client.BaseURL
	if raw, ok := argValue("--api"); ok {
		normalized, err := config.NormalizeAPI(raw)
		if err != nil {
			return err
		}
		target = normalized
	} else if add {
		target, _ = config.API()
	} else {
		target = authstore.ResolvedAPI()
	}
	client = api.NewClientFor(target)
	run := login.Run
	if add {
		run = login.RunAdd
	}
	me, err := run(client, func(userCode, verificationURL string) {
		fmt.Printf("Confirm this code in your browser: %s\n%s\n", userCode, verificationURL)
	})
	if err != nil {
		return err
	}
	fmt.Printf("Signed in as %s (%s). The daemon can now serve secrets.\n", me.Email, me.Organization)
	return nil
}

func runProfile() error {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "list" {
		return listProfiles()
	}
	if args[0] == "switch" {
		if len(args) < 2 {
			return fmt.Errorf("usage: kryptic profile switch <id|email>")
		}
		if err := login.Switch(args[1]); err != nil {
			return err
		}
		fmt.Printf("Switched to %s. Other accounts stay signed in.\n", args[1])
		return nil
	}
	if args[0] == "delete" || args[0] == "remove" {
		if len(args) < 2 {
			return fmt.Errorf("usage: kryptic profile delete <id|email>")
		}
		if !profileDeleteConfirmed() {
			fmt.Println("Delete cancelled.")
			return nil
		}
		if err := login.Delete(args[1]); err != nil {
			return err
		}
		fmt.Printf("Deleted %s from this install.\n", args[1])
		return nil
	}
	return fmt.Errorf("usage: kryptic profile [list|switch <id|email>|delete <id|email>]")
}

func listProfiles() error {
	store, err := authstore.LoadStore()
	if err != nil {
		fmt.Println("No saved accounts. Run `kryptic login`.")
		return nil
	}
	for _, p := range store.Profiles {
		mark := " "
		if p.ID == store.ActiveID {
			mark = "*"
		}
		state := "signed out"
		if p.SignedIn() {
			state = "signed in"
		}
		label := p.Email
		if label == "" {
			label = p.ID
		}
		if p.Organization != "" {
			label += " @ " + p.Organization
		}
		fmt.Printf("%s %s  (%s, %s, %s)\n", mark, label, state, p.API(), p.ID)
	}
	return nil
}

func runLogout(client *api.Client) error {
	if !logoutConfirmed() {
		fmt.Println("Logout cancelled - you are still signed in.")
		return nil
	}
	if err := login.Logout(client); err != nil {
		return err
	}
	fmt.Println("Signed out. This machine's encryption keys were kept, so the next")
	fmt.Println("`kryptic login` does not need a new admin grant unless durable device")
	fmt.Println("trust is off. Use `kryptic reset-device` to wipe the keys.")
	return nil
}

func runResetDevice(client *api.Client) error {
	all := hasArg("--all")
	if !resetDeviceConfirmed() {
		fmt.Println("Reset cancelled - this machine's keys are still here.")
		return nil
	}
	if all {
		if err := login.ResetAllDevices(client); err != nil {
			return err
		}
		fmt.Println("Every profile on this install was wiped. After the next")
		fmt.Println("`kryptic login`, an admin must grant the organization key under Approvals.")
		return nil
	}
	if err := login.ResetDevice(client); err != nil {
		return err
	}
	fmt.Println("This profile's keys were deleted. After your next `kryptic login`,")
	fmt.Println("an admin must grant the organization key to this machine under Approvals.")
	return nil
}

func hasArg(name string) bool {
	for _, arg := range os.Args[2:] {
		if arg == name {
			return true
		}
	}
	return false
}

func argValue(name string) (string, bool) {
	args := os.Args[2:]
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1], true
		}
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimPrefix(arg, name+"="), true
		}
	}
	return "", false
}

func profileDeleteConfirmed() bool {
	for _, arg := range os.Args[2:] {
		if arg == "--yes" || arg == "-y" {
			return true
		}
	}
	stat, err := os.Stdin.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice == 0 {
		return true
	}
	fmt.Println("This removes that account from this install and revokes its device on that server.")
	fmt.Println("Other profiles are not touched.")
	fmt.Print("Delete this profile? [y/N] ")
	var answer string
	_, _ = fmt.Scanln(&answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// logoutConfirmed warns that signing out discards the device's org-key grant
// and asks before proceeding. `--yes`/`-y` skips the prompt, and so does a
// non-interactive stdin (scripts, the menu-bar app shelling out to the CLI)
// where a prompt would hang.
func logoutConfirmed() bool {
	for _, arg := range os.Args[2:] {
		if arg == "--yes" || arg == "-y" {
			return true
		}
	}
	stat, err := os.Stdin.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice == 0 {
		return true
	}

	fmt.Println("Signing out revokes this session and drops secrets from memory.")
	fmt.Println("This machine's encryption keys stay, so the next sign-in does not")
	fmt.Println("need a new admin grant (unless durable device trust is off).")
	fmt.Print("Sign out anyway? [y/N] ")

	var answer string
	_, _ = fmt.Scanln(&answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func resetDeviceConfirmed() bool {
	for _, arg := range os.Args[2:] {
		if arg == "--yes" || arg == "-y" {
			return true
		}
	}
	stat, err := os.Stdin.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice == 0 {
		return true
	}

	if hasArg("--all") {
		fmt.Println("This deletes every profile's encryption keys on this install.")
	} else {
		fmt.Println("This deletes the active profile's encryption keys on this install.")
	}
	fmt.Println("The next `kryptic login` needs a new admin grant under Approvals.")
	fmt.Print("Reset this device? [y/N] ")

	var answer string
	_, _ = fmt.Scanln(&answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// platformAccessToken exchanges the stored refresh token for an access token,
// persisting the rotated refresh token (and keeping the device keys) in place.
func platformAccessToken(client *api.Client) (string, error) {
	var accessToken string
	err := authstore.WithLock(func() error {
		session, err := authstore.LoadSession()
		if err != nil {
			return err
		}
		if session.RefreshToken == "" {
			return authstore.ErrNotLoggedIn
		}
		tokens, err := client.Refresh(session.RefreshToken)
		if err != nil {
			applog.Error("cli", "auth.refresh", err, "result=error")
			return err
		}
		session.RefreshToken = tokens.RefreshToken
		if err := authstore.SaveSession(session); err != nil {
			applog.Error("cli", "auth.save", err, "result=error")
			return err
		}
		accessToken = tokens.AccessToken
		return nil
	})
	return accessToken, err
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
	if me.HasOrgKeyGrant != nil && !*me.HasOrgKeyGrant {
		fmt.Println("organization key: not granted (an admin must approve this device under Approvals)")
	}
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
		email, _ := response["email"].(string)
		org, _ := response["organization"].(string)
		if email == "" {
			fmt.Printf("daemon: online (v%v) - signed in\n", response["daemonVersion"])
		} else {
			fmt.Printf("daemon: online (v%v) - signed in as %s @ %s\n",
				response["daemonVersion"], email, org)
		}
		if granted, ok := response["orgKeyGranted"].(bool); ok && !granted {
			fmt.Println("organization key: not granted (an admin must approve this device under Approvals)")
		}
	} else {
		fmt.Printf("daemon: online (v%v) - not signed in (run `kryptic login`)\n", response["daemonVersion"])
	}
	if connection, ok := response["connection"].(string); ok && connection != "" {
		fmt.Printf("connection: %s\n", connection)
	}
	if raw, ok := response["profiles"].([]any); ok && len(raw) > 1 {
		fmt.Printf("profiles: %d saved (kryptic profile)\n", len(raw))
	}
	if reported, ok := response["apiUrl"].(string); ok && reported != "" {
		apiURL = reported
	}
	fmt.Printf("api: %s (%s)\n", apiURL, source)
	return nil
}

func runLogs() error {
	reveal := false
	for _, arg := range os.Args[2:] {
		if arg == "--reveal" || arg == "--open" {
			reveal = true
		}
	}
	text, err := applog.StatusLine()
	if err != nil {
		return err
	}
	fmt.Println(text)
	if reveal {
		return applog.Reveal()
	}
	return nil
}

func runConfig() error {
	args := os.Args[2:]
	if len(args) == 0 {
		url, source := config.Resolve(activeProfileAPI())
		fmt.Printf("api: %s (%s)\n", url, source)
		if config.EnvOverrides() {
			fmt.Println("KRYPTIC_API is set and overrides every profile's URL.")
		}
		return nil
	}
	switch args[0] {
	case "set-api":
		if len(args) < 2 {
			return fmt.Errorf("usage: kryptic config set-api URL")
		}
		return applyProfileAPI(args[1])
	case "reset-api":
		return applyProfileAPI(config.DefaultAPI)
	default:
		return fmt.Errorf("usage: kryptic config [set-api URL|reset-api]")
	}
}

func activeProfileAPI() string {
	store, err := authstore.LoadStore()
	if err != nil {
		return ""
	}
	p, ok := store.Active()
	if !ok {
		return ""
	}
	return p.Config.API
}

func applyProfileAPI(raw string) error {
	previous := authstore.ResolvedAPI()
	if err := login.SetActiveAPI(raw); err != nil {
		return err
	}
	next, source := config.Resolve(activeProfileAPI())
	fmt.Printf("api: %s (%s)\n", next, source)
	if config.EnvOverrides() {
		fmt.Println("KRYPTIC_API is set and still overrides the saved URL.")
		return nil
	}
	if previous != next {
		fmt.Println("this profile was signed out of the previous server. Other profiles are unchanged.")
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
	var skipped []string
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		key, _ := entry["key"].(string)
		value, _ := entry["value"].(string)
		if !envKeyPattern.MatchString(key) {
			skipped = append(skipped, key)
			continue
		}
		fmt.Println(dotenvLine(key, value))
	}
	warnSkippedKeys(skipped)
	return nil
}

// envKeyPattern is the POSIX shell identifier shape. A key outside it (spaces,
// newlines, `$(...)`) cannot be rendered safely on the left of a dotenv or
// shell `export` line: it would inject extra lines or commands into whatever
// sources the output. JSON export keeps every key - the encoder escapes them.
var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func warnSkippedKeys(skipped []string) {
	if len(skipped) == 0 {
		return
	}
	quoted := make([]string, len(skipped))
	for i, key := range skipped {
		quoted[i] = strconv.Quote(key)
	}
	fmt.Fprintf(os.Stderr, "warning: skipped %d key(s) that are not valid shell identifiers: %s\n",
		len(quoted), strings.Join(quoted, ", "))
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
