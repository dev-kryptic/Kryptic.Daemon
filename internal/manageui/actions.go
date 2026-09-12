package manageui

// Row is one management-window control: the same names the tray and CLI handle.
type Row struct {
	Name  string
	Arg   string
	Label string
}

func ActionRows(snap Snapshot) []Row {
	var rows []Row
	if snap.LoginInProgress {
		rows = append(rows, Row{Name: "cancelLogin", Label: "Cancel Sign-In"})
	} else if snap.Authenticated {
		rows = append(rows, Row{Name: "signOut", Label: "Sign Out"})
	} else {
		rows = append(rows, Row{Name: "signIn", Label: "Sign In"})
	}
	rows = append(rows, Row{Name: "addAccount", Label: "Add Account"})
	for _, profile := range snap.Profiles {
		if !profile.Active {
			rows = append(rows, Row{Name: "switchProfile", Arg: profile.ID, Label: profile.Title()})
		}
		name := profile.Email
		if name == "" {
			name = profile.ID
		}
		rows = append(rows, Row{Name: "deleteProfile", Arg: profile.ID, Label: "Delete " + name})
	}
	scan := "Scan for secrets"
	if snap.ScanInProgress {
		scan = "Scanning…"
	}
	update := snap.UpdateTitle
	if update == "" {
		update = "Check for Updates"
	}
	rows = append(rows,
		Row{Name: "flush", Label: "Refresh Secrets Cache"},
		Row{Name: "scan", Label: scan},
		Row{Name: "update", Label: update},
		Row{Name: "serverURI", Label: "Server URI"},
		Row{Name: "github", Label: "GitHub"},
		Row{Name: "docs", Label: "Docs"},
		Row{Name: "logs", Label: "Logs"},
		Row{Name: "about", Label: "About Kryptic"},
		Row{Name: "quit", Label: "Quit Kryptic"},
	)
	return rows
}

func KnownAction(name string) bool {
	switch name {
	case "signIn", "signOut", "cancelLogin", "addAccount", "switchProfile", "deleteProfile",
		"flush", "scan", "update", "serverURI", "github", "docs", "logs", "about", "quit":
		return true
	default:
		return false
	}
}
