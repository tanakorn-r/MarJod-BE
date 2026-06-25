#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────
# deploy.sh — Build, push, and deploy finance-chat to Cloud Run
# Usage: ./scripts/deploy.sh
# ─────────────────────────────────────────────────────────────
set -euo pipefail

# ── Config ────────────────────────────────────────────────────
GCP_PROJECT="marjod"
GCP_REGION="asia-southeast3"
REPO="finance-repo"
SERVICE="finance-chat"
IMAGE="${GCP_REGION}-docker.pkg.dev/${GCP_PROJECT}/${REPO}/${SERVICE}:latest"
ENV_FILE=".env.production"
ENV_YAML="/tmp/cloudrun-env.yaml"
TURSO_DB_NAME="finance-chat"

# ── Helpers ───────────────────────────────────────────────────
log() { echo "▶ $*"; }
die() { echo "✗ $*" >&2; exit 1; }

# ── Pre-flight checks ─────────────────────────────────────────
[[ -f "$ENV_FILE" ]] || die ".env file not found"
command -v docker  >/dev/null || die "docker not found"
command -v gcloud  >/dev/null || die "gcloud not found"

# ── Turso setup ───────────────────────────────────────────────
# Load current TURSO_URL from .env
TURSO_URL=$(grep -E '^TURSO_URL=' "$ENV_FILE" | cut -d= -f2- | tr -d '"' | tr -d "'")
TURSO_AUTH_TOKEN=$(grep -E '^TURSO_AUTH_TOKEN=' "$ENV_FILE" | cut -d= -f2- | tr -d '"' | tr -d "'")

if [[ -z "$TURSO_URL" ]]; then
  log "Turso not configured — setting up now..."
  command -v turso >/dev/null || die "turso CLI not found. Install: brew install tursodatabase/tap/turso && turso auth login"

  # Create DB if it doesn't exist
  if ! turso db show "$TURSO_DB_NAME" &>/dev/null; then
    log "Creating Turso database: $TURSO_DB_NAME"
    turso db create "$TURSO_DB_NAME" --location sin
  else
    log "Turso database '$TURSO_DB_NAME' already exists"
  fi

  # Get URL and token
  TURSO_URL=$(turso db show "$TURSO_DB_NAME" --url)
  TURSO_AUTH_TOKEN=$(turso db tokens create "$TURSO_DB_NAME")

  # Write back to .env
  # Replace or append TURSO_URL
  if grep -q '^TURSO_URL=' "$ENV_FILE"; then
    sed -i.bak "s|^TURSO_URL=.*|TURSO_URL=${TURSO_URL}|" "$ENV_FILE" && rm -f "${ENV_FILE}.bak"
  else
    echo "TURSO_URL=${TURSO_URL}" >> "$ENV_FILE"
  fi
  # Replace or append TURSO_AUTH_TOKEN
  if grep -q '^TURSO_AUTH_TOKEN=' "$ENV_FILE"; then
    sed -i.bak "s|^TURSO_AUTH_TOKEN=.*|TURSO_AUTH_TOKEN=${TURSO_AUTH_TOKEN}|" "$ENV_FILE" && rm -f "${ENV_FILE}.bak"
  else
    echo "TURSO_AUTH_TOKEN=${TURSO_AUTH_TOKEN}" >> "$ENV_FILE"
  fi

  log "Turso configured: $TURSO_URL"
else
  log "Turso already configured: $TURSO_URL"
fi

# ── Build env-vars YAML for Cloud Run ────────────────────────
# Using YAML avoids gcloud's comma-delimiter parsing issues with
# values that contain commas or special characters (e.g. CORS_ORIGINS).
log "Building env vars YAML from $ENV_FILE..."
echo "" > "$ENV_YAML"
while IFS= read -r line || [[ -n "$line" ]]; do
  # Skip blank lines and comments
  [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
  # Skip lines without '='
  [[ "$line" != *"="* ]] && continue
  key="${line%%=*}"
  value="${line#*=}"
  # Skip empty values and placeholder values
  [[ -z "$value" ]] && continue
  [[ "$value" == your_* ]] && continue
  # Skip reserved Cloud Run env vars
  [[ "$key" == "PORT" ]] && continue
  # Skip local-only vars (Cloud Run uses Turso, not local SQLite)
  [[ "$key" == "DB_PATH" ]] && continue
  # Write as YAML — quote the value to handle special chars
  echo "${key}: \"${value}\"" >> "$ENV_YAML"
done < "$ENV_FILE"

# ── Regenerate Swagger docs ───────────────────────────────────
log "Regenerating Swagger docs..."
$(go env GOPATH)/bin/swag init --generalInfo main.go --output docs --quiet 2>/dev/null || \
  log "Warning: swag not found, Swagger docs may be stale. Install: go install github.com/swaggo/swag/cmd/swag@latest"

# ── Authenticate Docker with Artifact Registry ────────────────
log "Authenticating Docker with Artifact Registry..."
gcloud auth configure-docker "${GCP_REGION}-docker.pkg.dev" --quiet

# ── Build for linux/amd64 and push ───────────────────────────
log "Building and pushing image: $IMAGE"
docker buildx build \
  --platform linux/amd64 \
  --tag "$IMAGE" \
  --push \
  .

# ── Deploy to Cloud Run ───────────────────────────────────────
log "Deploying to Cloud Run..."
gcloud run deploy "$SERVICE" \
  --image "$IMAGE" \
  --region "$GCP_REGION" \
  --platform managed \
  --allow-unauthenticated \
  --env-vars-file "$ENV_YAML" \
  --quiet

# ── Cleanup temp file ─────────────────────────────────────────
rm -f "$ENV_YAML"

# ── Print service URL ─────────────────────────────────────────
log "Done! Service URL:"
gcloud run services describe "$SERVICE" \
  --region "$GCP_REGION" \
  --format "value(status.url)"
