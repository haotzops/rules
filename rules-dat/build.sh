#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="${GITHUB_WORKSPACE:-$(pwd)}"
RELEASE_WORK="${RELEASE_WORK:?RELEASE_WORK is required}"
CONVERTER_DIR="${CONVERTER_DIR:-$ROOT_DIR/convert}"
BUILD_DIR="${RUNNER_TEMP:-/tmp}/v2ray-rules-dat-build"
RULES_DAT_DIR="$RELEASE_WORK/rules-dat"
GEO_DIR="$RULES_DAT_DIR/geo"
MIHOMO_DIR="$RULES_DAT_DIR/mihomo"
SING_BOX_DIR="$RULES_DAT_DIR/sing-box"
GEOSITE_UPSTREAM_URL="${V2RAY_RULES_DAT_URL:-https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release}"
GEOIP_UPSTREAM_URL="${LOYALSOLDIER_GEOIP_URL:-https://raw.githubusercontent.com/Loyalsoldier/geoip/release}"

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR" "$GEO_DIR" "$MIHOMO_DIR" "$SING_BOX_DIR"

fetch() {
  local url="$1"
  local output="$2"
  local temporary="${output}.tmp"

  mkdir -p "$(dirname "$output")"
  curl \
    --fail \
    --show-error \
    --silent \
    --location \
    --retry 3 \
    --retry-delay 2 \
    --retry-all-errors \
    --proto '=https' \
    --tlsv1.2 \
    "$url" \
    --output "$temporary"

  test -s "$temporary"
  mv "$temporary" "$output"
}

fetch "$GEOSITE_UPSTREAM_URL/geosite.dat" "$BUILD_DIR/geosite.dat"
fetch "$GEOIP_UPSTREAM_URL/geoip.dat" "$BUILD_DIR/geoip.dat"
fetch "$GEOIP_UPSTREAM_URL/Country.mmdb" "$BUILD_DIR/Country.mmdb"

go run ./rules-dat \
  --geosite-input "$BUILD_DIR/geosite.dat" \
  --geosite-output "$BUILD_DIR/geosite-lite.dat" \
  --geosite-categories "$ROOT_DIR/rules-dat/geosite-lite.txt" \
  --geoip-input "$BUILD_DIR/geoip.dat" \
  --geoip-output "$BUILD_DIR/geoip-lite.dat" \
  --geoip-countries CN,JP,US

export NO_SKIP=true
GOBIN="$BUILD_DIR/bin" go install -trimpath -ldflags='-s -w -buildid=' github.com/metacubex/geo/cmd/geo@master
GEO_BIN="$BUILD_DIR/bin/geo"

mkdir -p "$BUILD_DIR/meta-rule/geo/geosite" \
  "$BUILD_DIR/meta-rule/geo/geoip" \
  "$BUILD_DIR/meta-rule/geo-lite/geosite" \
  "$BUILD_DIR/meta-rule/geo-lite/geoip" \
  "$BUILD_DIR/sing-rule/geo/geosite" \
  "$BUILD_DIR/sing-rule/geo/geoip" \
  "$BUILD_DIR/sing-rule/geo-lite/geosite" \
  "$BUILD_DIR/sing-rule/geo-lite/geoip"

"$GEO_BIN" convert site -i v2ray -o sing -f "$BUILD_DIR/geosite.db" "$BUILD_DIR/geosite.dat"
"$GEO_BIN" convert site -i v2ray -o sing -f "$BUILD_DIR/geosite-lite.db" "$BUILD_DIR/geosite-lite.dat"
"$GEO_BIN" convert ip -i v2ray -o sing -f "$BUILD_DIR/geoip.db" "$BUILD_DIR/geoip.dat"
"$GEO_BIN" convert ip -i v2ray -o meta -f "$BUILD_DIR/geoip.metadb" "$BUILD_DIR/geoip.dat"
"$GEO_BIN" convert ip -i v2ray -o sing -f "$BUILD_DIR/geoip-lite.db" "$BUILD_DIR/geoip-lite.dat"
"$GEO_BIN" convert ip -i v2ray -o meta -f "$BUILD_DIR/geoip-lite.metadb" "$BUILD_DIR/geoip-lite.dat"

(
  cd "$CONVERTER_DIR"
  go run ./ geosite -f "$BUILD_DIR/geosite.dat" -o "$BUILD_DIR/sing-rule/geo/geosite" -t sing-box
  go run ./ geoip -f "$BUILD_DIR/geoip.dat" -o "$BUILD_DIR/sing-rule/geo/geoip" -t sing-box
  go run ./ geosite -f "$BUILD_DIR/geosite-lite.dat" -o "$BUILD_DIR/sing-rule/geo-lite/geosite" -t sing-box
  go run ./ geoip -f "$BUILD_DIR/geoip-lite.dat" -o "$BUILD_DIR/sing-rule/geo-lite/geoip" -t sing-box
  go run ./ geosite -f "$BUILD_DIR/geosite.dat" -o "$BUILD_DIR/meta-rule/geo/geosite"
  go run ./ geoip -f "$BUILD_DIR/geoip.dat" -o "$BUILD_DIR/meta-rule/geo/geoip"
  go run ./ geosite -f "$BUILD_DIR/geosite-lite.dat" -o "$BUILD_DIR/meta-rule/geo-lite/geosite"
  go run ./ geoip -f "$BUILD_DIR/geoip-lite.dat" -o "$BUILD_DIR/meta-rule/geo-lite/geoip"
)

while IFS= read -r -d '' file; do
  mv "$file" "${file%.list}.txt"
done < <(find "$BUILD_DIR/meta-rule" -type f -name '*.list' -print0)

cp "$BUILD_DIR/geosite.dat" "$GEO_DIR/"
cp "$BUILD_DIR/geosite-lite.dat" "$GEO_DIR/"
cp "$BUILD_DIR/geosite.db" "$GEO_DIR/"
cp "$BUILD_DIR/geosite-lite.db" "$GEO_DIR/"
cp "$BUILD_DIR/geoip.dat" "$GEO_DIR/"
cp "$BUILD_DIR/geoip-lite.dat" "$GEO_DIR/"
cp "$BUILD_DIR/geoip.db" "$GEO_DIR/"
cp "$BUILD_DIR/geoip-lite.db" "$GEO_DIR/"
cp "$BUILD_DIR/geoip.metadb" "$GEO_DIR/"
cp "$BUILD_DIR/geoip-lite.metadb" "$GEO_DIR/"
cp "$BUILD_DIR/Country.mmdb" "$GEO_DIR/"
cp -a "$BUILD_DIR/meta-rule/." "$MIHOMO_DIR/"
cp -a "$BUILD_DIR/sing-rule/." "$SING_BOX_DIR/"
cp "$ROOT_DIR/rules-dat/README_base.md" "$RULES_DAT_DIR/README_base.md"
cp "$ROOT_DIR/rules-dat/README.md" "$RULES_DAT_DIR/README.md"

while IFS= read -r -d '' file; do
  sha256sum "$file" > "$file.sha256sum"
done < <(find "$GEO_DIR" -type f ! -name '*.sha256sum' -print0)

printf 'built Geo data and rulesets in %s\n' "$RULES_DAT_DIR"
