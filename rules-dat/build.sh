#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="${GITHUB_WORKSPACE:-$(pwd)}"
RELEASE_WORK="${RELEASE_WORK:?RELEASE_WORK is required}"
CONVERTER_DIR="${CONVERTER_DIR:-$ROOT_DIR/convert}"
BUILD_DIR="${RUNNER_TEMP:-/tmp}/v2ray-rules-dat-build"
RULES_DAT_DIR="$RELEASE_WORK/rules-dat"
GEO_DIR="$RULES_DAT_DIR/geo"
CLASH_DOMAIN_DIR="$RULES_DAT_DIR/clash/domain"
CLASH_IPCIDR_DIR="$RULES_DAT_DIR/clash/ipcidr"
MIHOMO_GEOSITE_DIR="$RULES_DAT_DIR/mihomo/geosite"
MIHOMO_GEOIP_DIR="$RULES_DAT_DIR/mihomo/geoip"
SING_BOX_GEOSITE_DIR="$RULES_DAT_DIR/sing-box/geosite"
SING_BOX_GEOIP_DIR="$RULES_DAT_DIR/sing-box/geoip"
GEOSITE_UPSTREAM_URL="${V2RAY_RULES_DAT_URL:-https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release}"
GEOIP_UPSTREAM_URL="${LOYALSOLDIER_GEOIP_URL:-https://raw.githubusercontent.com/Loyalsoldier/geoip/release}"

rm -rf "$BUILD_DIR"
mkdir -p \
  "$BUILD_DIR" \
  "$GEO_DIR" \
  "$CLASH_DOMAIN_DIR" \
  "$CLASH_IPCIDR_DIR" \
  "$MIHOMO_GEOSITE_DIR" \
  "$MIHOMO_GEOIP_DIR" \
  "$SING_BOX_GEOSITE_DIR" \
  "$SING_BOX_GEOIP_DIR"

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

mkdir -p \
  "$BUILD_DIR/meta-rule/geo/geosite" \
  "$BUILD_DIR/meta-rule/geo/geoip" \
  "$BUILD_DIR/sing-rule/geo/geosite" \
  "$BUILD_DIR/sing-rule/geo/geoip"

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
  go run ./ geosite -f "$BUILD_DIR/geosite.dat" -o "$BUILD_DIR/meta-rule/geo/geosite"
  go run ./ geoip -f "$BUILD_DIR/geoip.dat" -o "$BUILD_DIR/meta-rule/geo/geoip"
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

prune_empty_rules() {
  local input_dir="$1"

  while IFS= read -r -d '' yaml_file; do
    local txt_file="${yaml_file%.yaml}.txt"
    if [[ ! -s "$txt_file" ]]; then
      rm -f "$yaml_file" "$txt_file" "${yaml_file%.yaml}.mrs"
    fi
  done < <(find "$input_dir" -maxdepth 1 -type f -name '*.yaml' -print0)
}

copy_rule_files() {
  local input_dir="$1"
  local output_dir="$2"

  find "$input_dir" -maxdepth 1 -type f \( -name '*.yaml' -o -name '*.txt' \) -exec cp {} "$output_dir/" \;
}

copy_sing_rule_files() {
  local input_dir="$1"
  local output_dir="$2"
  local source_dir="$3"

  while IFS= read -r -d '' txt_file; do
    local name
    name="$(basename "$txt_file" .txt)"
    cp "$input_dir/$name.json" "$output_dir/"
    cp "$input_dir/$name.srs" "$output_dir/"
  done < <(find "$source_dir" -maxdepth 1 -type f -name '*.txt' -print0)
}

# Empty upstream categories produce payload: [] files. Do not publish them.
prune_empty_rules "$BUILD_DIR/meta-rule/geo/geosite"
prune_empty_rules "$BUILD_DIR/meta-rule/geo/geoip"
prune_empty_rules "$BUILD_DIR/meta-rule/geo/geosite/classical"
prune_empty_rules "$BUILD_DIR/meta-rule/geo/geoip/classical"

# Domain/IP-CIDR files are the source formats used to build Mihomo MRS files.
copy_rule_files "$BUILD_DIR/meta-rule/geo/geosite" "$CLASH_DOMAIN_DIR"
copy_rule_files "$BUILD_DIR/meta-rule/geo/geoip" "$CLASH_IPCIDR_DIR"

# Mihomo's readable rules use classical syntax. MRS is generated separately
# from the domain/IP-CIDR sources because Mihomo does not support classical MRS.
copy_rule_files "$BUILD_DIR/meta-rule/geo/geosite/classical" "$MIHOMO_GEOSITE_DIR"
copy_rule_files "$BUILD_DIR/meta-rule/geo/geoip/classical" "$MIHOMO_GEOIP_DIR"

copy_sing_rule_files \
  "$BUILD_DIR/sing-rule/geo/geosite" \
  "$SING_BOX_GEOSITE_DIR" \
  "$BUILD_DIR/meta-rule/geo/geosite"
copy_sing_rule_files \
  "$BUILD_DIR/sing-rule/geo/geoip" \
  "$SING_BOX_GEOIP_DIR" \
  "$BUILD_DIR/meta-rule/geo/geoip"
cp "$ROOT_DIR/rules-dat/README_base.md" "$RULES_DAT_DIR/README_base.md"
cp "$ROOT_DIR/rules-dat/README.md" "$RULES_DAT_DIR/README.md"
cp "$ROOT_DIR/rules-dat/geosite-lite.txt" "$RULES_DAT_DIR/geosite-lite.txt"

while IFS= read -r -d '' file; do
  sha256sum "$file" > "$file.sha256sum"
done < <(find "$GEO_DIR" -type f ! -name '*.sha256sum' -print0)

printf 'built Geo data and rulesets in %s\n' "$RULES_DAT_DIR"
