package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dev-kryptic/daemon/internal/api"
	"github.com/dev-kryptic/daemon/internal/applog"
	"github.com/dev-kryptic/daemon/internal/pidfile"
	"github.com/dev-kryptic/daemon/internal/server"
)

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
		pidfile.Clear()
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
