package manageui

type panelAction struct {
	Name string `json:"name"`
	Arg  string `json:"arg"`
}

type panelSnap struct {
	Version         string         `json:"version"`
	Connection      string         `json:"connection"`
	ConnectionLabel string         `json:"connectionLabel"`
	Email           string         `json:"email"`
	Organization    string         `json:"organization"`
	APILabel        string         `json:"apiLabel"`
	Authenticated   bool           `json:"authenticated"`
	Running         bool           `json:"running"`
	CanLogin        bool           `json:"canLogin"`
	LoginInProgress bool           `json:"loginInProgress"`
	LoginCode       string         `json:"loginCode"`
	LoginError      string         `json:"loginError"`
	UpdateTitle     string         `json:"updateTitle"`
	ScanInProgress  bool           `json:"scanInProgress"`
	Profiles        []panelProfile `json:"profiles"`
}

type panelProfile struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Organization string `json:"organization"`
	APILabel     string `json:"apiLabel"`
	Active       bool   `json:"active"`
	SignedIn     bool   `json:"signedIn"`
}

func encodePanelSnap(snap Snapshot) panelSnap {
	version := snap.Version
	if version == "" {
		version = DefaultVersion()
	}
	out := panelSnap{
		Version:         version,
		Connection:      snap.Connection,
		ConnectionLabel: snap.ConnectionLabel,
		Email:           snap.Email,
		Organization:    snap.Organization,
		APILabel:        HostLabel(snap.API),
		Authenticated:   snap.Authenticated,
		Running:         snap.Running,
		CanLogin:        snap.CanLogin,
		LoginInProgress: snap.LoginInProgress,
		LoginCode:       snap.LoginCode,
		LoginError:      snap.LoginError,
		UpdateTitle:     snap.UpdateTitle,
		ScanInProgress:  snap.ScanInProgress,
	}
	for _, profile := range snap.Profiles {
		out.Profiles = append(out.Profiles, panelProfile{
			ID:           profile.ID,
			Email:        profile.Email,
			Organization: profile.Organization,
			APILabel:     HostLabel(profile.API),
			Active:       profile.Active,
			SignedIn:     profile.SignedIn,
		})
	}
	return out
}
