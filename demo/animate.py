#!/usr/bin/env python3
"""Assemble the three demo PNGs into one auto-looping animated WebP.

This is the README's "dynamic preview": a single <img> that cycles
list -> search -> detail in the browser with no JavaScript, so it works
inside a forge README (GitHub strips the JS/CSS a live switcher would
need, but they serve an animated image verbatim and the browser plays it).

WebP (not GIF) keeps the amber accent, env chips, and anti-aliased text
full-colour; GIF's 256-colour palette bands them. Run after `make shots`:

    python demo/animate.py            # -> demo/preview.webp

Requires Pillow (`pip install Pillow`).
"""
from __future__ import annotations

import sys
from pathlib import Path

from PIL import Image

HERE = Path(__file__).resolve().parent

# (frame file, dwell in ms). Detail lingers a touch longer — more to read.
FRAMES = [
    ("01-list.png", 2200),
    ("02-search.png", 2200),
    ("03-detail.png", 2800),
]

WIDTH = 1400  # display width of the WebP; source frames are 2304 wide (2x)
QUALITY = 90  # high enough to keep the monospace text crisp
OUT = HERE / "preview.webp"


def main() -> int:
    imgs: list[Image.Image] = []
    for name, _ in FRAMES:
        p = HERE / name
        if not p.exists():
            print(f"missing {p} (run 'make shots' first)", file=sys.stderr)
            return 1
        im = Image.open(p).convert("RGB")
        h = round(im.height * WIDTH / im.width)
        imgs.append(im.resize((WIDTH, h), Image.LANCZOS))

    imgs[0].save(
        OUT,
        format="WEBP",
        save_all=True,
        append_images=imgs[1:],
        duration=[d for _, d in FRAMES],
        loop=0,          # loop forever
        quality=QUALITY,
        method=6,        # slowest encoder pass = best compression
    )
    kb = OUT.stat().st_size / 1024
    print(f"wrote {OUT.name}  {imgs[0].width}x{imgs[0].height}  {len(imgs)} frames  {kb:.0f} KB")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
