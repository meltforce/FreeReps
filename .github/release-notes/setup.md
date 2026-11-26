## Server Setup

**Fresh install:**

```bash
git clone https://github.com/meltforce/FreeReps.git
cd FreeReps
cp server/config.example.yaml server/config.yaml
# Edit config.yaml — set the database password, choose Tailscale or application login
cd server && docker compose up -d
```

**Upgrade existing server:**

```bash
cd server
docker compose pull
docker compose up -d
```

## Upload Tool (macOS)

**One-liner install:**

```bash
curl -sSL https://raw.githubusercontent.com/meltforce/FreeReps/main/server/scripts/install-upload.sh | bash
```

**Or download manually** from the assets below and place in `/usr/local/bin/`.

Requires `lzfse`: `brew install lzfse`
