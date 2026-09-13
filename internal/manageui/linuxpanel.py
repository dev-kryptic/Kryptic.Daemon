#!/usr/bin/env python3
# Open Kryptic for Linux. Layout matches the macOS/Windows panel: 380px
# column, status, account rows, operations, settings, help.
import json
import sys
import threading

import gi

def _load_gtk():
    last = None
    # Pin Gdk to the same series before importing Gtk. Newer Ubuntu loads
    # Gdk 4 by default, which then conflicts with Gtk 3.
    for ver in ("3.0", "4.0"):
        try:
            gi.require_version("Gdk", ver)
            gi.require_version("Gtk", ver)
            gi.require_version("GdkPixbuf", "2.0")
            return ver
        except ValueError as err:
            last = err
    raise SystemExit("GTK 3 or 4 is required: %s" % last)


GTK_VER = _load_gtk()
GTK4 = GTK_VER.startswith("4")
from gi.repository import Gdk, GdkPixbuf, GLib, Gtk, Pango


LIGHT = """
window { background: #ffffff; }
label { color: #1a1a1a; }
.muted { color: #80726b; }
.faint { color: #afa39c; font-size: 11px; font-weight: 600; letter-spacing: 0.6px; }
.danger { color: #e11d48; }
.k-btn { border-radius: 10px; border: 1px solid #dde6e0; background: #f4f4f5; color: #1a1a1a; min-height: 32px; font-weight: 600; font-size: 13px; }
.k-btn:hover { background: #ececec; }
.k-primary { background: #16b069; color: #04140c; border-color: #16b069; }
.k-primary:hover { background: #25c078; }
.k-danger { background: #eee8fd; color: #e11d48; border-color: #eee8fd; }
.k-danger:hover { background: #e0d4f8; }
.k-row { border-radius: 10px; border: 1px solid #dde6e0; background: #f4f4f5; min-height: 46px; }
.k-row-active { border-color: #16b069; }
.k-trash { border-radius: 10px; border: 1px solid #dde6e0; background: #f4f4f5; color: #e11d48; min-width: 34px; min-height: 34px; }
.rule { background: #e0e6dd; min-height: 1px; }
"""

DARK = """
window { background: #202020; }
label { color: #f0f0f0; }
.muted { color: #b8ada6; }
.faint { color: #8e827a; font-size: 11px; font-weight: 600; letter-spacing: 0.6px; }
.danger { color: #ff453a; }
.k-btn { border-radius: 10px; border: 1px solid #3a3a3a; background: #2c2c2e; color: #f0f0f0; min-height: 32px; font-weight: 600; font-size: 13px; }
.k-btn:hover { background: #353535; }
.k-primary { background: #5ff2a6; color: #04140c; border-color: #5ff2a6; }
.k-primary:hover { background: #78f5b5; }
.k-danger { background: #282838; color: #ff453a; border-color: #282838; }
.k-danger:hover { background: #303042; }
.k-row { border-radius: 10px; border: 1px solid #3a3a3a; background: #2c2c2e; min-height: 46px; }
.k-row-active { border-color: #5ff2a6; }
.k-trash { border-radius: 10px; border: 1px solid #3a3a3a; background: #2c2c2e; color: #ff453a; min-width: 34px; min-height: 34px; }
.rule { background: #3a3a3a; min-height: 1px; }
"""

DOT = {
    "connected": "#30d158",
    "connecting": "#ff9f0a",
    "awaiting_approval": "#ff9f0a",
}


def emit(name, arg=""):
    sys.stdout.write(json.dumps({"name": name, "arg": arg}) + "\n")
    sys.stdout.flush()


def add_class(widget, name):
    if GTK4:
        widget.add_css_class(name)
    else:
        widget.get_style_context().add_class(name)


def remove_class(widget, name):
    if GTK4:
        widget.remove_css_class(name)
    else:
        widget.get_style_context().remove_class(name)


def box_add(box, child, expand=False):
    if GTK4:
        box.append(child)
        if expand:
            child.set_hexpand(True)
    else:
        box.pack_start(child, expand, expand, 0)


def box_clear(box):
    if GTK4:
        child = box.get_first_child()
        while child is not None:
            nxt = child.get_next_sibling()
            box.remove(child)
            child = nxt
        return
    for child in list(box.get_children()):
        box.remove(child)


def set_child(parent, child):
    if GTK4:
        parent.set_child(child)
    else:
        parent.add(child)


def scaled_logo(path, size=40):
    if not path:
        return None
    try:
        pixbuf = GdkPixbuf.Pixbuf.new_from_file_at_scale(path, size, size, True)
    except Exception:
        return None
    image = Gtk.Image.new_from_pixbuf(pixbuf)
    image.set_size_request(size, size)
    image.set_halign(Gtk.Align.START)
    image.set_valign(Gtk.Align.CENTER)
    return image


def label(text="", muted=False):
    widget = Gtk.Label(label=text, xalign=0)
    if muted:
        add_class(widget, "muted")
    return widget


def dark_theme():
    settings = Gtk.Settings.get_default()
    if settings is not None and settings.get_property("gtk-application-prefer-dark-theme"):
        return True
    name = ""
    if settings is not None:
        name = (settings.get_property("gtk-theme-name") or "").lower()
    return "dark" in name


def apply_css():
    provider = Gtk.CssProvider()
    css = DARK if dark_theme() else LIGHT
    if hasattr(provider, "load_from_string"):
        provider.load_from_string(css)
    else:
        provider.load_from_data(css.encode())
    if GTK4:
        Gtk.StyleContext.add_provider_for_display(
            Gdk.Display.get_default(), provider, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION
        )
        return
    Gtk.StyleContext.add_provider_for_screen(
        Gdk.Screen.get_default(), provider, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION
    )


def button(text, kind="surface"):
    btn = Gtk.Button(label=text)
    if not GTK4:
        btn.set_relief(Gtk.ReliefStyle.NONE)
    add_class(btn, "k-btn")
    if kind == "primary":
        add_class(btn, "k-primary")
    elif kind == "danger":
        add_class(btn, "k-danger")
    return btn


def set_kind(btn, kind):
    remove_class(btn, "k-primary")
    remove_class(btn, "k-danger")
    if kind == "primary":
        add_class(btn, "k-primary")
    elif kind == "danger":
        add_class(btn, "k-danger")


def section(title):
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
    rule = Gtk.Box()
    rule.set_size_request(-1, 1)
    add_class(rule, "rule")
    box_add(box, rule)
    head = label(title)
    add_class(head, "faint")
    box_add(box, head)
    return box


class Panel(Gtk.Window):
    def __init__(self, logo_path):
        super().__init__(title="Open Kryptic")
        self.set_default_size(380, 640)
        self.set_size_request(380, 480)
        self.set_resizable(True)
        self._loop = None
        if GTK4:
            self.connect("close-request", self._on_close)
        else:
            self.connect("destroy", Gtk.main_quit)

        root = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        root.set_margin_top(16)
        root.set_margin_bottom(16)
        root.set_margin_start(16)
        root.set_margin_end(16)
        root.set_size_request(348, -1)

        header = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=12)
        image = scaled_logo(logo_path)
        if image is not None:
            box_add(header, image)
        titles = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        box_add(titles, label("Kryptic"))
        self.version = label("", muted=True)
        box_add(titles, self.version)
        box_add(header, titles, expand=True)
        box_add(root, header)

        status = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        self.dot = Gtk.Label(xalign=0)
        self.status = label("Signed out")
        box_add(row, self.dot)
        box_add(row, self.status, expand=True)
        self.email = label("")
        self.org = label("", muted=True)
        self.api = label("", muted=True)
        self.banner = label("")
        self.banner.set_line_wrap(True)
        box_add(status, row)
        box_add(status, self.email)
        box_add(status, self.org)
        box_add(status, self.api)
        box_add(status, self.banner)
        box_add(root, status)

        box_add(root, section("ACCOUNT"))
        self.account = button("Sign In", "primary")
        self.account.connect("clicked", lambda *_: self.on_account())
        self.add_account = button("Add Account")
        self.add_account.connect("clicked", lambda *_: emit("addAccount"))
        self.profiles = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        box_add(root, self.account)
        box_add(root, self.add_account)
        box_add(root, self.profiles)

        box_add(root, section("OPERATIONS"))
        self.flush = button("Refresh Secrets Cache")
        self.flush.connect("clicked", lambda *_: emit("flush"))
        self.scan = button("Scan for secrets")
        self.scan.connect("clicked", lambda *_: emit("scan"))
        box_add(root, self.flush)
        box_add(root, self.scan)

        box_add(root, section("SETTINGS"))
        self.update = button("Check for Updates")
        self.update.connect("clicked", lambda *_: emit("update"))
        self.server = button("Server URI")
        self.server.connect("clicked", lambda *_: emit("serverURI"))
        box_add(root, self.update)
        box_add(root, self.server)

        box_add(root, section("HELP"))
        help_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        self.github = button("GitHub")
        self.docs = button("Docs")
        self.logs = button("Logs")
        self.github.connect("clicked", lambda *_: emit("github"))
        self.docs.connect("clicked", lambda *_: emit("docs"))
        self.logs.connect("clicked", lambda *_: emit("logs"))
        box_add(help_row, self.github, expand=True)
        box_add(help_row, self.docs, expand=True)
        box_add(help_row, self.logs, expand=True)
        self.about = button("About Kryptic")
        self.about.connect("clicked", lambda *_: emit("about"))
        self.quit_btn = button("Quit Kryptic", "danger")
        self.quit_btn.connect("clicked", lambda *_: emit("quit"))
        box_add(root, help_row)
        box_add(root, self.about)
        box_add(root, self.quit_btn)

        scroll = Gtk.ScrolledWindow()
        scroll.set_policy(Gtk.PolicyType.NEVER, Gtk.PolicyType.AUTOMATIC)
        set_child(scroll, root)
        set_child(self, scroll)
        self.snap = {}
        if GTK4:
            self.present()
        else:
            self.show_all()

    def _on_close(self, *_args):
        if self._loop is not None:
            self._loop.quit()
        return False

    def run(self):
        if GTK4:
            self._loop = GLib.MainLoop()
            self._loop.run()
            return
        Gtk.main()

    def on_account(self):
        snap = self.snap
        if snap.get("loginInProgress"):
            emit("cancelLogin")
        elif snap.get("authenticated"):
            emit("signOut")
        else:
            emit("signIn")

    def apply(self, snap):
        self.snap = snap
        version = snap.get("version") or ""
        self.version.set_text("Version " + version if version else "")
        kind = snap.get("connection") or "signed_out"
        color = DOT.get(kind, "#8e8e93")
        self.dot.set_markup('<span foreground="%s">●</span>' % color)
        self.status.set_text(snap.get("connectionLabel") or "Signed out")
        self.email.set_text(snap.get("email") or "")
        self.org.set_text(snap.get("organization") or "")
        self.api.set_text(snap.get("apiLabel") or "")
        if snap.get("loginCode"):
            self.banner.set_text("Confirm code in browser: " + snap["loginCode"])
            remove_class(self.banner, "danger")
        elif snap.get("loginError"):
            self.banner.set_text(snap["loginError"])
            add_class(self.banner, "danger")
        else:
            self.banner.set_text("")
            remove_class(self.banner, "danger")

        if snap.get("loginInProgress"):
            self.account.set_label("Cancel Sign-In")
            set_kind(self.account, "surface")
            self.account.set_sensitive(True)
        elif snap.get("authenticated"):
            self.account.set_label("Sign Out")
            set_kind(self.account, "danger")
            self.account.set_sensitive(True)
        else:
            self.account.set_label("Sign In")
            set_kind(self.account, "primary")
            self.account.set_sensitive(bool(snap.get("canLogin")))

        self.add_account.set_sensitive(bool(snap.get("canLogin")) and not snap.get("loginInProgress"))
        self.flush.set_sensitive(bool(snap.get("running")))
        if snap.get("scanInProgress"):
            self.scan.set_label("Scanning…")
            self.scan.set_sensitive(False)
        else:
            self.scan.set_label("Scan for secrets")
            self.scan.set_sensitive(bool(snap.get("canLogin")))
        self.update.set_label(snap.get("updateTitle") or "Check for Updates")
        self.update.set_sensitive(bool(snap.get("canLogin")))
        self.server.set_sensitive(bool(snap.get("canLogin")))

        box_clear(self.profiles)
        for profile in snap.get("profiles") or []:
            box_add(self.profiles, self.profile_row(profile))
        if not GTK4:
            self.profiles.show_all()
        return False

    def profile_row(self, profile):
        row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=6)
        btn = Gtk.Button()
        if not GTK4:
            btn.set_relief(Gtk.ReliefStyle.NONE)
        inner = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        check = Gtk.Label(label="✓" if profile.get("active") else "", xalign=0)
        check.set_width_chars(1)
        texts = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=1)
        name = label(profile.get("email") or profile.get("id") or "")
        name.set_ellipsize(Pango.EllipsizeMode.END)
        meta_parts = []
        if profile.get("organization"):
            meta_parts.append(profile["organization"])
        if profile.get("apiLabel"):
            meta_parts.append(profile["apiLabel"])
        if not profile.get("signedIn"):
            meta_parts.append("signed out")
        meta = label(" · ".join(meta_parts), muted=True)
        meta.set_ellipsize(Pango.EllipsizeMode.END)
        box_add(texts, name)
        if meta_parts:
            box_add(texts, meta)
        box_add(inner, check)
        box_add(inner, texts, expand=True)
        set_child(btn, inner)
        add_class(btn, "k-row")
        if profile.get("active"):
            add_class(btn, "k-row-active")
        btn.set_sensitive(not profile.get("active"))
        pid = profile.get("id") or ""
        btn.connect("clicked", lambda *_: emit("switchProfile", pid))

        trash = Gtk.Button(label="🗑")
        if not GTK4:
            trash.set_relief(Gtk.ReliefStyle.NONE)
        add_class(trash, "k-trash")
        trash.connect("clicked", lambda *_: emit("deleteProfile", pid))
        box_add(row, btn, expand=True)
        box_add(row, trash)
        return row


def read_snaps(panel):
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            snap = json.loads(line)
        except json.JSONDecodeError:
            continue
        GLib.idle_add(panel.apply, snap)


def main():
    apply_css()
    logo = sys.argv[1] if len(sys.argv) > 1 else ""
    panel = Panel(logo)
    threading.Thread(target=read_snaps, args=(panel,), daemon=True).start()
    panel.run()


if __name__ == "__main__":
    main()
