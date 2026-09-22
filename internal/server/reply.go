package server

import (
	"encoding/json"
	"net"

	"github.com/dev-kryptic/daemon/internal/orgkey"
)

// Typed protocol replies. Maps of any box every bool and string on the heap
// for json.Marshal; these structs keep the PROTOCOL.md shape without that.
type failReply struct {
	V       int    `json:"v"`
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type okReply struct {
	V  int  `json:"v"`
	OK bool `json:"ok"`
}

type flushReply struct {
	V       int  `json:"v"`
	OK      bool `json:"ok"`
	Cleared int  `json:"cleared"`
}

type secretsReply struct {
	V       int           `json:"v"`
	OK      bool          `json:"ok"`
	Secrets []orgkey.Pair `json:"secrets"`
}

type profileInfo struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Organization string `json:"organization"`
	API          string `json:"api"`
	Active       bool   `json:"active"`
	SignedIn     bool   `json:"signedIn"`
}

type statusReply struct {
	V               int           `json:"v"`
	OK              bool          `json:"ok"`
	Authenticated   bool          `json:"authenticated"`
	Email           string        `json:"email,omitempty"`
	Organization    string        `json:"organization,omitempty"`
	DaemonVersion   string        `json:"daemonVersion"`
	APIUrl          string        `json:"apiUrl"`
	OrgKeyGranted   bool          `json:"orgKeyGranted"`
	Connection      string        `json:"connection"`
	ActiveProfileId string        `json:"activeProfileId,omitempty"`
	Profiles        []profileInfo `json:"profiles"`
}

func fail(code, message string) failReply {
	return failReply{V: protocolVersion, Error: code, Message: message}
}

func (s *Server) reply(connection net.Conn, payload any) {
	data, _ := json.Marshal(payload)
	_, _ = connection.Write(append(data, '\n'))
}
