// Package ipc is the single place that knows how the daemon and its clients talk
// on each platform: a unix domain socket on macOS/Linux, a named pipe on Windows
// (PROTOCOL.md §Transport). The KRYPTIC_SOCKET_PATH env var overrides either.
package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"time"
)

// Request performs one NDJSON round-trip against the running daemon.
func Request(payload map[string]any) (map[string]any, error) {
	connection, err := Dial(2 * time.Second)
	if err != nil {
		return nil, errors.New("daemon is not running - start it with `kryptic start`")
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))

	payload["v"] = 1
	data, _ := json.Marshal(payload)
	if _, err := connection.Write(append(data, '\n')); err != nil {
		return nil, err
	}

	line, err := bufio.NewReader(connection).ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var response map[string]any
	return response, json.Unmarshal(line, &response)
}

var _ = net.Conn(nil)
