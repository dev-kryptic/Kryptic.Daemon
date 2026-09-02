package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"fyne.io/systray"
	"github.com/dev-kryptic/daemon/internal/dialog"
	"github.com/dev-kryptic/daemon/internal/scan"
	"github.com/dev-kryptic/daemon/internal/server"
)

var folderScanBusy atomic.Bool

func runFolderScan(item *systray.MenuItem) {
	if !folderScanBusy.CompareAndSwap(false, true) {
		return
	}
	item.Disable()
	defer func() {
		folderScanBusy.Store(false)
		item.Enable()
		item.SetTitle("Scan…")
	}()

	folder, ok := dialog.PickFolder("Scan folder")
	if !ok {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	progress := dialog.OpenProgress("Kryptic", "Scanning…")
	defer progress.Close()

	go func() {
		select {
		case <-progress.Canceled():
			cancel()
		case <-ctx.Done():
		}
	}()

	result, err := scan.ScanFolder(ctx, folder, func(percent int, message string) {
		item.SetTitle(fmt.Sprintf("Scanning… %d%%", percent))
		progress.Set(percent, message)
	}, server.Version)
	progress.Close()

	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		dialog.Info("Kryptic", "Scan failed: "+err.Error())
		return
	}

	message := fmt.Sprintf(
		"%d files scanned.\n%d potential secret(s) found.\n\nReport:\n%s\n\nThis scan ran fully offline. Nothing left this machine.",
		result.Files, len(result.Findings), result.ReportPath,
	)
	if dialog.Ask("Kryptic", message, "Open Report", "Close") {
		dialog.OpenPath(result.ReportPath)
	}
}
