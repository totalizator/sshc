# sshc demo tape for VHS (Linux / macOS) — https://github.com/charmbracelet/vhs
#
# Renders demo/sshc.gif: a ~17s browse/search showcase that loops cleanly
# (search + env chips → detail panel → pin/unpin → live theming).
#
# Native Windows VHS currently hangs at the headless-browser stage (go-rod), so
# this is the tape to fire on a Linux/macOS box (or a container). Two ways:
#
#   A) Local vhs (deps: vhs, ttyd, ffmpeg, a chromium/chrome):
#        GOOS=linux GOARCH=amd64 go build -o sshc .   # a Linux sshc on PATH/CWD
#        vhs demo/sshc.linux.tape
#
#   B) Official VHS Docker image (bundles ttyd+ffmpeg+chromium — most reliable):
#        GOOS=linux GOARCH=amd64 go build -o sshc .
#        docker run --rm -v "$PWD":/vhs ghcr.io/charmbracelet/vhs demo/sshc.linux.tape
#      (the image's workdir is /vhs; `sshc` below resolves from there)
#
# Run from the repo root so demo/ssh_config resolves. The hidden Setup records
# against a throwaway copy of the bundled sample config and a temp
# XDG_CONFIG_HOME, so a run never mutates the committed sample or your real sshc
# settings, and the look is deterministic each run.

Output demo/sshc.gif

Set Shell bash
Set FontSize 13
Set Width 1000
Set Height 640
Set Padding 14
Set TypingSpeed 70ms

# --- Setup (hidden): isolate settings + config, then clear the screen ---
Hide
Type "export XDG_CONFIG_HOME=$(mktemp -d)" Enter
Type "cp demo/ssh_config /tmp/sshc-demo.conf" Enter
Type "clear" Enter
Show

# --- Launch (the command doubles as usage doc) ---
Type@40ms "sshc --config /tmp/sshc-demo.conf"
Sleep 400ms
Enter
Sleep 2s

# --- Scene 1: fuzzy search — typing 'edge' filters instantly and matches the
#     env label, showing the coloured env chips ---
Type@110ms "edge"
Sleep 1.8s
Escape
Sleep 600ms

# --- Scene 2: detail panel — Tab reveals every field + the resolved ssh command ---
Tab
Sleep 2.2s
Tab
Sleep 500ms

# --- Scene 3: pin/unpin — Ctrl+F drops the selected favourite into ALL HOSTS and
#     back into ★ PINNED; the selection follows the host both ways ---
Ctrl+F
Sleep 1.8s
Ctrl+F
Sleep 1.5s

# --- Scene 4: live appearance — Style → rich (per-env gutter bars), then a live
#     accent change ---
Ctrl+T
Sleep 700ms
Right
Sleep 1.3s
Down
Right
Sleep 1.3s
Escape
Sleep 800ms

# --- Quit ---
Ctrl+C
Sleep 400ms

# --- Teardown (hidden) ---
Hide
Type "rm -rf /tmp/sshc-demo.conf" Enter
