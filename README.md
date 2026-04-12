# Atlas Tones 🔔

![Banner](banner-image.png)

A fast, metadata-driven repository and CLI for iPhone ringtones and notification sounds. Managed by `gobake` and `piml`.

## Features
- **Categorized Sounds:** Sounds are grouped by category (e.g., Age of Empires 2, Civ 6).
- **PIML Metadata:** Easy-to-manage sound descriptions and types using the PIML format.
- **Beautiful CLI UI:** Uses `pterm` for a gorgeous, interactive, and colorful terminal experience.
- **Cross-Platform:** Built with Go, works on Windows, macOS, and Linux.
- **Easy Installation:** Guided installation process to get sounds onto your iPhone.
- **Auto Conversion:** Built-in MP3 to M4R (AAC) conversion using FFmpeg.

## Prerequisites
- **FFmpeg:** Required for on-the-fly MP3 to M4R conversion.
- **iTunes / Finder:** Required for syncing to non-jailbroken iPhones.

## Usage

### Display Help
```bash
atlas.tones
```
*Shows a beautiful list of available commands and descriptions.*

### List all sounds
```bash
atlas.tones list
```
*Displays a formatted table of all available sounds and categories.*

### Update Catalog
```bash
atlas.tones sync
```
*Fetches the latest sound registry from the cloud.*

### Prepare a sound (or full category) for installation
```bash
# Install a specific sound
atlas.tones install "Age of Empires 2" Wololo

# Install all sounds in a category
atlas.tones install "Age of Empires 2"
```
*This downloads the sounds, converts them, opens your file manager, and gives you a drag-and-drop prompt for iTunes/Finder.*

## Adding New Sounds (For Maintainers)
1. Create a new directory in your CDN or local registry source.
2. Add your `.mp3` or `.m4r` files.
3. Update the registry `manifest.piml` following this template:

```piml
(category) Category Name
(description) Short description.

(sound)
  > (item)
    (title) Sound Title
    (file) filename.m4r
    (type) ringtone
    (source) https://url.to/file.m4r
```

## Building
Requires `gobake`.
```bash
gobake build
```
