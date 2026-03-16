#!/usr/bin/env python3
"""Generate framed App Store screenshots using Apple iPhone bezels."""

from PIL import Image, ImageDraw, ImageFont
from pathlib import Path

# Paths
BASE = Path(__file__).parent
SCREENSHOTS = BASE / "screenshots"
BEZELS = BASE / "bezels"
OUTPUT = BASE / "framed"
OUTPUT.mkdir(exist_ok=True)

# Canvas size (App Store 6.9")
W, H = 1320, 2868

# Bezel dimensions (iPhone 17 Pro Max)
BEZEL_W, BEZEL_H = 1470, 3000
SCREEN_OFFSET_X = 75   # (1470-1320)/2
SCREEN_OFFSET_Y = 66   # (3000-2868)/2

# FreeReps brand colors
ACCENT = (0, 113, 227)      # #0071e3 — website accent
BLACK = (0, 0, 0)
WHITE = (255, 255, 255)
GRAY = (102, 102, 102)
GRAY_DARK = (136, 136, 136)


def load_font(size, weight_name="Regular"):
    font_path = "/System/Library/Fonts/SFNS.ttf"
    try:
        f = ImageFont.truetype(font_path, size)
        f.set_variation_by_name(weight_name)
        return f
    except (OSError, IOError, ValueError):
        return ImageFont.load_default()


FONT_HEADLINE = load_font(82, "Bold")
FONT_SUBTEXT = load_font(44, "Semibold")

# Screenshot definitions
# (output_name, screenshot_file, headline_parts, subtext, bg_color)
SHOTS = [
    ("01-main", "main_screen.png",
     [("Sync", True), (" your health data", False)],
     "85+ HealthKit data types", "white"),

    ("02-sync", "sync.png",
     [("Real-time", True), (" sync progress", False)],
     "Background sync keeps you up to date", "white"),

    ("03-settings", "settings.png",
     [("Easy ", False), ("configuration", True)],
     "", "white"),

    ("04-permissions", "health_permissions.png",
     [("Full ", False), ("HealthKit", True), (" access", False)],
     "", "white"),
]

DEVICE_SCALE = 0.60
DEVICE_VPOS = 0.52


def draw_headline(draw, parts, y, is_white):
    normal_color = BLACK if is_white else WHITE

    total_w = 0
    for text, _ in parts:
        bbox = draw.textbbox((0, 0), text, font=FONT_HEADLINE)
        total_w += bbox[2] - bbox[0]

    x = (W - total_w) // 2
    for text, is_accent in parts:
        color = ACCENT if is_accent else normal_color
        draw.text((x, y), text, font=FONT_HEADLINE, fill=color)
        bbox = draw.textbbox((0, 0), text, font=FONT_HEADLINE)
        x += bbox[2] - bbox[0]


def generate(name, screenshot_file, headline_parts, subtext, bg):
    screenshot_path = SCREENSHOTS / screenshot_file
    if not screenshot_path.exists():
        print(f"  SKIP {name}: {screenshot_file} not found")
        return False

    bezel_path = BEZELS / "iPhone 17 Pro Max - Silver - Portrait.png"
    if not bezel_path.exists():
        print(f"  SKIP {name}: bezel not found")
        return False

    is_white = bg == "white"
    bg_color = WHITE if is_white else BLACK

    canvas = Image.new("RGB", (W, H), bg_color)

    bezel = Image.open(bezel_path).convert("RGBA")
    screenshot = Image.open(screenshot_path).convert("RGB")

    # Scale
    bezel_draw_w = int(BEZEL_W * DEVICE_SCALE)
    bezel_draw_h = int(BEZEL_H * DEVICE_SCALE)
    screen_draw_w = int(1320 * DEVICE_SCALE)
    screen_draw_h = int(2868 * DEVICE_SCALE)

    bezel_resized = bezel.resize((bezel_draw_w, bezel_draw_h), Image.LANCZOS)
    screenshot_resized = screenshot.resize((screen_draw_w, screen_draw_h), Image.LANCZOS)

    # Position
    bezel_x = (W - bezel_draw_w) // 2
    bezel_y = int(H * DEVICE_VPOS - bezel_draw_h / 2)
    screen_x = bezel_x + int(SCREEN_OFFSET_X * DEVICE_SCALE)
    screen_y = bezel_y + int(SCREEN_OFFSET_Y * DEVICE_SCALE)

    # Paste screenshot, then bezel on top
    canvas.paste(screenshot_resized, (screen_x, screen_y))

    bezel_layer = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    bezel_layer.paste(bezel_resized, (bezel_x, bezel_y))
    canvas = Image.composite(bezel_layer, canvas.convert("RGBA"), bezel_layer).convert("RGB")

    # Draw text
    draw = ImageDraw.Draw(canvas)

    # Headline: centered between top and device top
    headline_bbox = draw.textbbox((0, 0), "Xg", font=FONT_HEADLINE)
    headline_h = headline_bbox[3] - headline_bbox[1]
    headline_y = bezel_y // 2 - headline_h // 2
    draw_headline(draw, headline_parts, headline_y, is_white)

    # Subtext: centered between device bottom and canvas bottom
    if subtext:
        sub_color = GRAY if is_white else GRAY_DARK
        bbox = draw.textbbox((0, 0), subtext, font=FONT_SUBTEXT)
        sub_w = bbox[2] - bbox[0]
        sub_h = bbox[3] - bbox[1]
        device_bottom = bezel_y + bezel_draw_h
        sub_x = (W - sub_w) // 2
        sub_y = device_bottom + (H - device_bottom) // 2 - sub_h // 2
        draw.text((sub_x, sub_y), subtext, font=FONT_SUBTEXT, fill=sub_color)

    out_path = OUTPUT / f"{name}.png"
    canvas.save(out_path, "PNG")
    return True


def main():
    print(f"Generating {len(SHOTS)} framed screenshots...")
    print(f"Output: {OUTPUT}")
    print()

    ok = 0
    for shot in SHOTS:
        name = shot[0]
        success = generate(*shot)
        if success:
            ok += 1
            print(f"  OK  {name}.png")

    print(f"\nDone: {ok}/{len(SHOTS)} screenshots generated in {OUTPUT}")


if __name__ == "__main__":
    main()
