# Rasterize demo/*.svg -> demo/*.png with headless Chrome/Edge.
#
# GitHub sanitize an SVG's <style> block at render time, dropping the
# embedded font sizing and chip colours, so the SVGs look broken on the forge even
# though the files are correct. The README therefore ships PNGs. Run this after
# `make shots`:
#
#     powershell -NoProfile -File demo/rasterize.ps1
#
# IMPORTANT: rasterize at the SVG's *natural* size (1152x578 for the 128x34 shot
# at the Makefile's 9x17 cell). Chrome rescales the page to the --window-size; a
# non-integer scale (e.g. forcing a 1152px-wide SVG into a 990px window) reopens a
# ~1px gap between each host's two rows. Match the window to the SVG and the grid
# tiles pixel-perfectly.
#
# (On Linux/macOS, rasterize with `rsvg-convert` or `chromium --headless
# --screenshot` instead — same 1152x578 canvas.)
$ErrorActionPreference = 'Stop'
$browser = @(
  "$env:ProgramFiles\Google\Chrome\Application\chrome.exe",
  "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe",
  "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe"
) | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $browser) { throw "No Chrome/Edge found to rasterize with." }

$dir = Split-Path -Parent $PSCommandPath
foreach ($name in '01-list','02-search','03-detail') {
  $svg = Join-Path $dir "$name.svg"
  $png = Join-Path $dir "$name.png"
  if (-not (Test-Path $svg)) { Write-Warning "missing $svg (run 'make shots' first)"; continue }
  if (Test-Path $png) { Remove-Item $png -Force }
  $url = "file:///" + ($svg -replace '\\','/')
  $a = @('--headless=new','--disable-gpu','--hide-scrollbars','--force-device-scale-factor=2',
         '--window-size=1152,578', "--screenshot=$png", $url)
  $p = Start-Process -FilePath $browser -ArgumentList $a -NoNewWindow -PassThru
  if (-not $p.WaitForExit(30000)) { Stop-Process -Id $p.Id -Force; throw "$name timed out" }
  if (Test-Path $png) { "rendered $name.png" } else { throw "$name produced no PNG" }
}
