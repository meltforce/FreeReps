#!/usr/bin/env python3
"""Turn raw iPhone captures into the inputs of generate-screenshots.py.

    python3 release/prepare-screenshots.py <capture-dir>

<capture-dir> holds the device captures (xcrun devicectl device capture
screenshot, 1320 x 2868) named <shot>-light.png and <shot>-dark.png for the
shots dashboard, syncing, settings and widget. They stay outside the repo: the
settings capture shows the real server host, which names the tailnet.

What changes on the way to release/screenshots/:
- settings: the host line becomes the default placeholder host and the server
  version line a release number. The text is redrawn in the app's own font and
  colours rather than blacked out.
- widget: the widget and the FreeReps icon are cut from the Home Screen
  capture and placed on a rebuilt Home Screen with a plain gradient, so
  neither other apps' icons nor the wallpaper or the device language appear
  in a listing. Only the clock of the status bar is redrawn.
- dashboard, syncing: copied unchanged.
"""

import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

OUT = Path(__file__).parent / "screenshots"

PLACEHOLDER_HOST = "freereps.your-tailnet.ts.net"
SERVER_LINE = "Connected · Server 2.1.0"

# Pixel boxes on a 1320 x 2868 capture of the Settings tab, measured on the
# 2026-09-25 captures: the host line, and the status line right of the green dot.
HOST_BOX = (313, 586, 1010, 633)
STATUS_BOX = (353, 653, 1117, 689)
CARD_BG_SAMPLE = (1150, 560)

# The small widget on a Home Screen capture: first row, first two columns,
# measured where the widget meets the wallpaper on the light capture.
WIDGET_BOX = (76, 282, 625, 831)
# How far into the box the corner shape reaches.
CORNER = 140
# The FreeReps icon on the same capture (fourth column, third row), and the
# Home Screen slot it moves to: right of the widget, as the first icon.
ICON_BOX = (1006, 900, 1245, 1139)
ICON_SLOT = (695, 282)

# Home Screen gradients, top to bottom, in the app's navy and teal.
WALLPAPER = {
    "light": ((0xD4, 0xEE, 0xE8), (0xEE, 0xF1, 0xF7)),
    "dark": ((0x0F, 0x1A, 0x30), (0x07, 0x2E, 0x2A)),
}
HOME_TEXT = {"light": (0x1C, 0x1C, 0x1E), "dark": (255, 255, 255)}


def font(size, weight):
    f = ImageFont.truetype("/System/Library/Fonts/SFNS.ttf", size)
    f.set_variation_by_name(weight)
    return f


def fitted_font(text, width, weight):
    """The SF size at which `text` is `width` pixels wide."""
    size = 20
    while font(size + 1, weight).getlength(text) <= width:
        size += 1
    return font(size, weight)


def text_color(im, box, bg):
    """The ink colour of the text in `box`: the pixel furthest from the background."""
    px = im.load()
    best, best_d = bg, -1
    for y in range(box[1], box[3]):
        for x in range(box[0], box[2]):
            p = px[x, y]
            d = sum(abs(p[i] - bg[i]) for i in range(3))
            if d > best_d:
                best, best_d = p, d
    return best


def replace_text(im, box, old_text, new_text, weight, bg):
    color = text_color(im, box, bg)
    f = fitted_font(old_text, box[2] - box[0], weight)
    draw = ImageDraw.Draw(im)
    draw.rectangle((box[0] - 4, box[1] - 6, 1140, box[3] + 6), fill=bg)
    # Align the new text's ink top with the old one's.
    top_offset = f.getbbox(new_text)[1]
    draw.text((box[0], box[1] - top_offset), new_text, font=f, fill=color)


def prepare_settings(src, dst):
    im = Image.open(src).convert("RGB")
    bg = im.getpixel(CARD_BG_SAMPLE)
    replace_text(im, HOST_BOX, "freereps.coydog-fence.ts.net", PLACEHOLDER_HOST, "Semibold", bg)
    replace_text(im, STATUS_BOX, "Connected · Server edge-0373318daf54…", SERVER_LINE, "Regular", bg)
    im.save(dst)


def corner_mask(light_capture):
    """The widget's outline as an alpha mask, taken from the light capture.

    iOS draws widgets with continuous corners, which no circular radius matches;
    a guessed radius left wallpaper in the corners. On the light capture the
    widget is white on a dark wallpaper, so the outline of the top-left corner is
    where each row turns white, scanning inwards; the pixel on the edge gets its
    brightness as coverage. Only the outline is read, so text near a corner stays
    opaque. The other corners mirror it, and the dark widget sits at the same
    place and uses the same mask.
    """
    im = Image.open(light_capture).convert("L").crop(WIDGET_BOX)
    w, h = im.size
    wall = im.getpixel((0, 0))
    edge = []  # per row of the corner: (first opaque x, coverage of the pixel before it)
    for y in range(CORNER):
        x = 0
        while x < CORNER and im.getpixel((x, y)) < 250:
            x += 1
        before = im.getpixel((x - 1, y)) if x > 0 else 255
        edge.append((x, max(0, min(255, round((before - wall) * 255 / (255 - wall))))))

    mask = Image.new("L", (w, h), 255)
    for dy, (x_in, partial) in enumerate(edge):
        for dx in range(x_in):
            alpha = partial if dx == x_in - 1 else 0
            for x, y in ((dx, dy), (w - 1 - dx, dy), (dx, h - 1 - dy), (w - 1 - dx, h - 1 - dy)):
                mask.putpixel((x, y), alpha)
    return mask


def gradient(size, top, bottom):
    w, h = size
    im = Image.new("RGB", size)
    draw = ImageDraw.Draw(im)
    for y in range(h):
        t = y / (h - 1)
        draw.line(((0, y), (w, y)), fill=tuple(round(top[i] + (bottom[i] - top[i]) * t) for i in range(3)))
    return im


def icon_mask(size):
    """iOS's icon outline, close enough at this size: a rounded square."""
    scale = 4
    big = Image.new("L", (size * scale, size * scale), 0)
    ImageDraw.Draw(big).rounded_rectangle((0, 0, size * scale - 1, size * scale - 1),
                                          radius=round(size * 0.2237 * scale), fill=255)
    return big.resize((size, size), Image.LANCZOS)


def prepare_home(src, dst, mask, mode):
    """A Home Screen holding only the FreeReps widget and the FreeReps icon."""
    capture = Image.open(src).convert("RGBA")
    home = gradient(capture.size, *WALLPAPER[mode]).convert("RGBA")

    widget = capture.crop(WIDGET_BOX)
    widget.putalpha(mask)
    home.alpha_composite(widget, WIDGET_BOX[:2])

    icon = capture.crop(ICON_BOX)
    icon.putalpha(icon_mask(icon.width))
    home.alpha_composite(icon, ICON_SLOT)

    draw = ImageDraw.Draw(home)
    label = font(36, "Medium")
    x = ICON_SLOT[0] + (icon.width - label.getlength("FreeReps")) / 2
    draw.text((x, ICON_SLOT[1] + icon.height + 14), "FreeReps", font=label, fill=HOME_TEXT[mode])
    draw.text((160, 72), "9:41", font=font(51, "Semibold"), fill=HOME_TEXT[mode])
    home.convert("RGB").save(dst)


def main():
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    src = Path(sys.argv[1])
    OUT.mkdir(exist_ok=True)
    mask = corner_mask(src / "widget-light.png")
    for mode in ("light", "dark"):
        for shot in ("dashboard", "syncing"):
            Image.open(src / f"{shot}-{mode}.png").convert("RGB").save(OUT / f"{shot}-{mode}.png")
        prepare_settings(src / f"settings-{mode}.png", OUT / f"settings-{mode}.png")
        prepare_home(src / f"widget-{mode}.png", OUT / f"widget-{mode}.png", mask, mode)
        print(f"prepared {mode}")


if __name__ == "__main__":
    main()
