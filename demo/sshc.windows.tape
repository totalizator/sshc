# sshc demo tape for VHS (Windows PowerShell) — https://github.com/charmbracelet/vhs
#
# Same ~17s showcase as demo/sshc.linux.tape, but with a PowerShell shell and
# Windows-native Setup (%APPDATA% override isolates the Ctrl+T settings write;
# sshc stores settings under %APPDATA% on Windows).
#
# ⚠ KNOWN ISSUE: as of VHS 0.11.0, native Windows rendering hangs at the
# headless-browser (go-rod) stage — ttyd serves fine and Chrome launches, but the
# DevTools handshake never completes, so `vhs demo\sshc.windows.tape` freezes
# right after the `Set` lines. See charmbracelet/vhs#437, #528, #85. Until that is
# fixed upstream, render demo/sshc.gif on Linux/macOS with demo/sshc.linux.tape
# (or the official VHS Docker image). This tape is kept ready for when Windows
# support works.
#
# Prerequisites (once VHS Windows works): vhs on PATH, .\sshc.exe built
# (`make build`), run from the repo root:  vhs demo\sshc.windows.tape

Output demo/sshc.gif

Set Shell powershell
Set FontSize 13
Set Width 1000
Set Height 640
Set Padding 14
Set TypingSpeed 70ms

# --- Setup (hidden): isolate settings + config, then clear the screen ---
Hide
Type '$env:APPDATA = "$env:TEMP\sshc-demo-cfg"'
Enter
Type 'Copy-Item demo\ssh_config "$env:TEMP\sshc-demo.conf" -Force'
Enter
Type 'Clear-Host'
Enter
Show

# --- Launch (the command doubles as usage doc) ---
Type@40ms '.\sshc.exe --config "$env:TEMP\sshc-demo.conf"'
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
Type 'Remove-Item -Recurse -Force "$env:TEMP\sshc-demo.conf","$env:TEMP\sshc-demo-cfg" -ErrorAction SilentlyContinue'
Enter
