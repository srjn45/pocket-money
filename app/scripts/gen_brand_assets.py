#!/usr/bin/env python3
"""
Idempotent generator for Pocket Money branding PNGs.

Usage:
    python3 app/scripts/gen_brand_assets.py

Requires: Pillow (already present in repo tooling env)
No package.json entry added — per WP-4.5 spec §2.3 D4.

Outputs:
    app/assets/icon.png          1024×1024 opaque RGB
    app/assets/adaptive-icon.png 1024×1024 transparent RGBA (safe-zone glyph only)
    app/assets/splash-icon.png   1024×1024 transparent RGBA (rounded tile + glyph)
    app/assets/favicon.png         48×48   opaque RGB
"""

import os
import math
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

BRAND_INDIGO = "#4F46E5"
WHITE = "#FFFFFF"
FONT_PATH = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"
RUPEE = "₹"

ASSETS_DIR = Path(__file__).parent.parent / "assets"


def _find_font(size: int) -> ImageFont.FreeTypeFont:
    for path in [
        FONT_PATH,
        "/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
    ]:
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    raise RuntimeError("No suitable font found; install DejaVu Sans or Noto Sans.")


def _draw_centered_text(draw: ImageDraw.ImageDraw, canvas: int, font_size: int, color: str) -> None:
    font = _find_font(font_size)
    draw.text((canvas // 2, canvas // 2), RUPEE, font=font, fill=color, anchor="mm")


def _add_rounded_rect(draw: ImageDraw.ImageDraw, xy, radius: int, fill: str) -> None:
    x0, y0, x1, y1 = xy
    draw.rounded_rectangle([x0, y0, x1, y1], radius=radius, fill=fill)


def gen_icon() -> None:
    """1024×1024 opaque — full-bleed indigo tile, white ₹ at ~55% canvas."""
    size = 1024
    img = Image.new("RGB", (size, size), BRAND_INDIGO)
    draw = ImageDraw.Draw(img)
    _draw_centered_text(draw, size, int(size * 0.55), WHITE)
    out = ASSETS_DIR / "icon.png"
    img.save(out, "PNG", optimize=False)
    print(f"  wrote {out}  ({img.size[0]}×{img.size[1]} {img.mode})")


def gen_adaptive_icon() -> None:
    """1024×1024 transparent — white ₹ at ~45% canvas, inside 66% safe zone."""
    size = 1024
    img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    _draw_centered_text(draw, size, int(size * 0.45), WHITE)
    out = ASSETS_DIR / "adaptive-icon.png"
    img.save(out, "PNG", optimize=False)
    print(f"  wrote {out}  ({img.size[0]}×{img.size[1]} {img.mode})")


def gen_splash_icon() -> None:
    """1024×1024 transparent — rounded indigo tile (~70% canvas, r≈18%), white ₹."""
    size = 1024
    tile = int(size * 0.70)         # 716 px tile
    pad = (size - tile) // 2        # centering offset
    radius = int(tile * 0.18)       # ≈129 px corner radius

    img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    _add_rounded_rect(draw, (pad, pad, pad + tile, pad + tile), radius, BRAND_INDIGO)
    _draw_centered_text(draw, size, int(tile * 0.60), WHITE)
    out = ASSETS_DIR / "splash-icon.png"
    img.save(out, "PNG", optimize=False)
    print(f"  wrote {out}  ({img.size[0]}×{img.size[1]} {img.mode})")


def gen_favicon() -> None:
    """48×48 opaque — downscaled icon (indigo tile + white ₹)."""
    size = 48
    img = Image.new("RGB", (size, size), BRAND_INDIGO)
    draw = ImageDraw.Draw(img)
    _draw_centered_text(draw, size, int(size * 0.55), WHITE)
    out = ASSETS_DIR / "favicon.png"
    img.save(out, "PNG", optimize=False)
    print(f"  wrote {out}  ({img.size[0]}×{img.size[1]} {img.mode})")


if __name__ == "__main__":
    print("Generating Pocket Money brand assets...")
    gen_icon()
    gen_adaptive_icon()
    gen_splash_icon()
    gen_favicon()
    print("Done.")
