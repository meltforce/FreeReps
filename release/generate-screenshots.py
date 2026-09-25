#!/usr/bin/env python3
"""Generate framed App Store screenshots, light and dark.

Reads release/screenshots/<shot>-<mode>.png (written by prepare-screenshots.py)
and writes:
- release/framed/<nn>-<shot>-<mode>.png, the App Store set (6.9", 1320 x 2868)
- docs/screenshots/ios/framed-<shot>.png and framed-<shot>-dark.png, the same
  images for the website and the app README
"""

import shutil
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

BASE = Path(__file__).parent
SCREENSHOTS = BASE / "screenshots"
BEZELS = BASE / "bezels"
OUTPUT = BASE / "framed"
DOCS = BASE.parent / "docs" / "screenshots" / "ios"

# Canvas size (App Store 6.9")
W, H = 1320, 2868

# Bezel dimensions (iPhone 17 Pro Max)
BEZEL_W, BEZEL_H = 1470, 3000
SCREEN_OFFSET_X = 75   # (1470-1320)/2
SCREEN_OFFSET_Y = 66   # (3000-2868)/2

# Per mode: background, headline, accent (the app's Brand colour), subtext.
MODES = {
    "light": {"bg": (255, 255, 255), "text": (0, 0, 0), "accent": (0x0E, 0x80, 0x6F), "sub": (102, 102, 102)},
    "dark": {"bg": (0, 0, 0), "text": (255, 255, 255), "accent": (0x3C, 0xCF, 0xB6), "sub": (136, 136, 136)},
}


def load_font(size, weight_name="Regular"):
    font_path = "/System/Library/Fonts/SFNS.ttf"
    try:
        f = ImageFont.truetype(font_path, size)
        f.set_variation_by_name(weight_name)
        return f
    except (OSError, IOError, ValueError):
        return ImageFont.load_default()


FONT_SUBTEXT = load_font(44, "Semibold")

# (number, shot, headline parts, subtext, kind)
# Every capture goes into the bezel; the widget shot is a rebuilt Home Screen.
SHOTS = [
    ("01", "dashboard",
     [("Sync", True), (" your health data", False)],
     "Only what changed since the last sync", "phone"),

    ("02", "syncing",
     [("Every sync, ", False), ("visible", True)],
     "Progress per category while it runs", "phone"),

    ("03", "widget",
     [("Sync from your ", False), ("Home Screen", True)],
     "Tap the widget, or ask Siri", "phone"),

    ("04", "settings",
     [("Your server, ", False), ("your data", True)],
     "Self-hosted, reached over your tailnet", "phone"),
]

# Corner radius of the display in capture pixels. A capture is rectangular and
# the bezel's screen opening is not; unrounded, the capture's corners showed
# outside the frame.
SCREEN_RADIUS = 186

DEVICE_SCALE = 0.60
DEVICE_VPOS = 0.52


def headline_font(parts):
    """82 pt bold, smaller when the headline would not fit the canvas."""
    text = "".join(t for t, _ in parts)
    size = 82
    while size > 50 and load_font(size, "Bold").getlength(text) > W - 120:
        size -= 2
    return load_font(size, "Bold")


def draw_headline(draw, parts, y, colors):
    font = headline_font(parts)
    total_w = sum(font.getlength(text) for text, _ in parts)
    x = (W - total_w) / 2
    for text, is_accent in parts:
        draw.text((x, y), text, font=font, fill=colors["accent"] if is_accent else colors["text"])
        x += font.getlength(text)


def phone_layer(screenshot):
    """The capture inside the bezel, and the device's top and bottom on the canvas."""
    bezel = Image.open(BEZELS / "iPhone 17 Pro Max - Silver - Portrait.png").convert("RGBA")
    bezel_w, bezel_h = int(BEZEL_W * DEVICE_SCALE), int(BEZEL_H * DEVICE_SCALE)
    screen_w, screen_h = int(1320 * DEVICE_SCALE), int(2868 * DEVICE_SCALE)
    bezel_x = (W - bezel_w) // 2
    bezel_y = int(H * DEVICE_VPOS - bezel_h / 2)

    screen = screenshot.convert("RGBA")
    mask = Image.new("L", screen.size, 0)
    ImageDraw.Draw(mask).rounded_rectangle((0, 0, screen.width - 1, screen.height - 1), radius=SCREEN_RADIUS, fill=255)
    screen.putalpha(mask)

    layer = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    layer.alpha_composite(screen.resize((screen_w, screen_h), Image.LANCZOS),
                        (bezel_x + int(SCREEN_OFFSET_X * DEVICE_SCALE), bezel_y + int(SCREEN_OFFSET_Y * DEVICE_SCALE)))
    layer.alpha_composite(bezel.resize((bezel_w, bezel_h), Image.LANCZOS), (bezel_x, bezel_y))
    return layer, bezel_y, bezel_y + bezel_h


def generate(number, shot, headline_parts, subtext, kind, mode):
    source = SCREENSHOTS / f"{shot}-{mode}.png"
    if not source.exists():
        print(f"  SKIP {shot}-{mode}: {source.name} not found")
        return None
    colors = MODES[mode]
    canvas = Image.new("RGBA", (W, H), colors["bg"] + (255,))
    capture = Image.open(source)
    layer, top, bottom = phone_layer(capture.convert("RGB"))
    canvas.alpha_composite(layer)

    draw = ImageDraw.Draw(canvas)
    box = draw.textbbox((0, 0), "Xg", font=headline_font(headline_parts))
    draw_headline(draw, headline_parts, top // 2 - (box[3] - box[1]) // 2, colors)

    if subtext:
        box = draw.textbbox((0, 0), subtext, font=FONT_SUBTEXT)
        draw.text(((W - (box[2] - box[0])) // 2, bottom + (H - bottom) // 2 - (box[3] - box[1]) // 2),
                  subtext, font=FONT_SUBTEXT, fill=colors["sub"])

    out = OUTPUT / f"{number}-{shot}-{mode}.png"
    canvas.convert("RGB").save(out, "PNG")
    return out


def main():
    OUTPUT.mkdir(exist_ok=True)
    DOCS.mkdir(parents=True, exist_ok=True)
    ok = 0
    for number, shot, parts, subtext, kind in SHOTS:
        for mode in MODES:
            out = generate(number, shot, parts, subtext, kind, mode)
            if out is None:
                continue
            suffix = "" if mode == "light" else "-dark"
            shutil.copyfile(out, DOCS / f"framed-{shot}{suffix}.png")
            ok += 1
            print(f"  OK  {out.name}")
    print(f"\nDone: {ok}/{len(SHOTS) * len(MODES)} screenshots in {OUTPUT} and {DOCS}")


if __name__ == "__main__":
    main()
