#!/usr/bin/env bash
# 重新拉取北京 POI 快照（OpenStreetMap，经 Overpass API）。只在需要更新快照时手动运行，CI 不会运行它。
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
out="${1:-$repo_root/data/raw/beijing_pois_osm.json}"
endpoint="${OVERPASS_URL:-https://overpass-api.de/api/interpreter}"
# Overpass 会拒绝没有 User-Agent 的请求（HTTP 406）。
user_agent="beijing-geo-map (https://github.com/slatt-oo/beijing-geo-map)"
min_elements=1000

query='[out:json][timeout:180];
area["ISO3166-2"="CN-BJ"]["admin_level"="4"]->.bj;
(
  nwr["railway"="station"]["station"="subway"](area.bj);
  nwr["public_transport"="station"]["subway"="yes"](area.bj);
  nwr["amenity"~"^(university|hospital)$"]["name"](area.bj);
  nwr["tourism"~"^(attraction|museum)$"]["name"](area.bj);
  nwr["leisure"="park"]["name"](area.bj);
);
out center tags;'

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

validate_snapshot() {
  python3 - "$1" "$min_elements" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1]))
remark = doc.get("remark")
if remark:
    sys.exit(f"Overpass reported an incomplete result: {remark}")
count = len(doc.get("elements", []))
if count < int(sys.argv[2]):
    sys.exit(f"Overpass returned only {count} elements (need at least {sys.argv[2]})")
print(count)
PY
}

for delay in 30 60 120 0; do
  code="$(curl -sS -m 240 -A "$user_agent" --data-urlencode "data=$query" -o "$tmp" -w '%{http_code}' "$endpoint" || true)"
  if [ "$code" = 200 ]; then
    if count="$(validate_snapshot "$tmp")"; then
      mv "$tmp" "$out"
      chmod 644 "$out"
      trap - EXIT
      echo "saved $count elements to $out"
      exit 0
    fi
  else
    echo "Overpass returned HTTP $code" >&2
  fi
  if [ "$delay" -eq 0 ]; then
    break
  fi
  echo "retrying in ${delay}s..." >&2
  sleep "$delay"
done

echo "failed to fetch POIs from Overpass; the existing snapshot was left unchanged" >&2
exit 1
