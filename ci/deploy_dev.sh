#!/usr/bin/env bash
set -euo pipefail

required_vars=(
  GCP_PROJECT_ID
  GCP_REGION
  GCP_SERVICE_NAME_DEV
  GCP_SA_KEY_B64
)

for name in "${required_vars[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    echo "Missing required CI variable: ${name}" >&2
    exit 1
  fi
done

key_file="$(mktemp)"
cleanup() {
  rm -f "$key_file"
}
trap cleanup EXIT

printf '%s' "$GCP_SA_KEY_B64" | base64 -d > "$key_file"

gcloud auth activate-service-account --key-file="$key_file"
gcloud config set project "$GCP_PROJECT_ID"

args=(
  run deploy "$GCP_SERVICE_NAME_DEV"
  --source .
  --region "$GCP_REGION"
  --project "$GCP_PROJECT_ID"
  --quiet
)

if [[ "${CLOUD_RUN_ALLOW_UNAUTHENTICATED:-true}" == "true" ]]; then
  args+=(--allow-unauthenticated)
fi

if [[ -n "${CLOUD_RUN_ENV_VARS:-}" ]]; then
  args+=(--set-env-vars "$CLOUD_RUN_ENV_VARS")
fi

gcloud "${args[@]}"

service_url="$(gcloud run services describe "$GCP_SERVICE_NAME_DEV" --region "$GCP_REGION" --project "$GCP_PROJECT_ID" --format='value(status.url)')"
echo "Deployed to: ${service_url}"
