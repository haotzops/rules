#!/usr/bin/env bash

set -euo pipefail

: "${COMMUNITY_DIR:?COMMUNITY_DIR is required}"

DATA_DIR="$COMMUNITY_DIR/data-lite"
TEMP_DIR="${RUNNER_TEMP:-/tmp}"
V2FLY_ATTRIBUTE_DATA_DIR="$TEMP_DIR/geosite-lite-v2fly-data"
V2FLY_ATTRIBUTE_OUTPUT_DIR="$TEMP_DIR/geosite-lite-v2fly-output"
LITE_OUTPUT_DIR="$TEMP_DIR/geosite-lite-output"
mkdir -p "$DATA_DIR"
rm -f "$DATA_DIR"/*

fetch() {
  local url="$1"
  local output="$2"
  local temporary="${output}.tmp"

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

yaml_domains() {
  grep DOMAIN "$1" |
    grep -v '#' |
    sed 's/  - DOMAIN,/full:/g' |
    sed 's/  - DOMAIN-SUFFIX,//g' |
    sed 's/  - DOMAIN-KEYWORD,/keyword:/g'
}

# Copy a v2ray-format source from v2fly/domain-list-community together with
# its include: dependencies, so the lite compilation can resolve them.
copy_v2fly() {
  local name="$1"
  local destination="${2:-$DATA_DIR}"
  local src="$COMMUNITY_DIR/data/$name"
  local dst="$destination/$name"

  mkdir -p "$destination"

  if [ ! -f "$src" ]; then
    echo "error: v2fly data/$name not found" >&2
    return 1
  fi
  if [ ! -e "$dst" ]; then
    cp "$src" "$dst"
  fi

  local dep
  while IFS= read -r dep; do
    copy_v2fly "$dep" "$destination"
  done < <(grep '^include:' "$src" | sed -E 's/^include:([^@[:space:]:]+).*/\1/')
}

# Merge attributes from the canonical v2fly lists without replacing the
# broader BlackMatrix7 domain coverage used by the lite lists.
merge_v2fly_attributes() {
  local source target
  local -a sources=()
  local -a targets=()

  rm -rf "$V2FLY_ATTRIBUTE_DATA_DIR" "$V2FLY_ATTRIBUTE_OUTPUT_DIR"
  mkdir -p "$V2FLY_ATTRIBUTE_DATA_DIR" "$V2FLY_ATTRIBUTE_OUTPUT_DIR"

  while IFS='|' read -r source target; do
    copy_v2fly "$source" "$V2FLY_ATTRIBUTE_DATA_DIR"
    sources+=("$source")
    targets+=("$target")
  done <<'SOURCES'
abema|abema
apple|apple
apple-music|applemusic
bilibili|bilibili
bahamut|bahamut
cloudflare|cloudflare
google|google
github|github
microsoft|microsoft
netflix|netflix
openai|openai
onedrive|onedrive
pixiv|pixiv
spotify|spotify
telegram|telegram
twitter|twitter
tiktok|tiktok
youtube|youtube
icloud|icloud
instagram|instagram
linkedin|linkedin
steam|steam
epicgames|epicgames
SOURCES

  local export_lists
  export_lists=$(IFS=,; printf '%s' "${sources[*]}")
  (
    cd "$COMMUNITY_DIR"
    go run ./ \
      --datapath="$V2FLY_ATTRIBUTE_DATA_DIR" \
      --outputdir="$V2FLY_ATTRIBUTE_OUTPUT_DIR" \
      --outputname v2fly-attributes.dat \
      --exportlists="$export_lists"
  )

  local index source_file target_file
  for index in "${!sources[@]}"; do
    source_file="$V2FLY_ATTRIBUTE_OUTPUT_DIR/${sources[$index]}.txt"
    target_file="$DATA_DIR/${targets[$index]}"
    test -f "$source_file"
    grep -E ':[@][[:alnum:]_!-]+' "$source_file" |
      sed -E 's/:((@[[:alnum:]_!-]+)(,@[[:alnum:]_!-]+)*)$/ \1/; s/,@/ @/g' >> "$target_file" || true
  done
}

fetch \
  "https://raw.githubusercontent.com/xishang0128/rules/main/biliintl.list" \
  "$DATA_DIR/biliintl"
fetch \
  "https://github.com/v2fly/domain-list-community/raw/master/data/ehentai" \
  "$DATA_DIR/ehentai"
fetch \
  "https://github.com/v2fly/domain-list-community/raw/master/data/private" \
  "$DATA_DIR/private"

while IFS='|' read -r source name; do
  fetch \
    "https://github.com/blackmatrix7/ios_rule_script/raw/master/rule/Clash/$source.yaml" \
    "$TEMP_DIR/${name}.yaml"
  yaml_domains "$TEMP_DIR/${name}.yaml" > "$DATA_DIR/$name"
done <<'SOURCES'
AbemaTV/AbemaTV|abema
Apple/Apple_Classical|apple
AppleMusic/AppleMusic|applemusic
BiliBili/BiliBili|bilibili
Bahamut/Bahamut|bahamut
Cloudflare/Cloudflare|cloudflare
Google/Google|google
GitHub/GitHub|github
Microsoft/Microsoft|microsoft
Netflix/Netflix|netflix
OpenAI/OpenAI|openai
OneDrive/OneDrive|onedrive
Pixiv/Pixiv|pixiv
ProxyLite/ProxyLite|proxy
Spotify/Spotify|spotify
Telegram/Telegram|telegram
Twitter/Twitter|twitter
TikTok/TikTok|tiktok
YouTube/YouTube|youtube
GlobalMedia/GlobalMedia|proxymedia
iCloud/iCloud|icloud
Instagram/Instagram|instagram
LinkedIn/LinkedIn|linkedin
Steam/Steam|steam
Epic/Epic|epicgames
SOURCES

merge_v2fly_attributes

fetch \
  "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/China/China_Domain.yaml" \
  "$TEMP_DIR/China_Domain.yaml"
grep - \
  "$TEMP_DIR/China_Domain.yaml" |
  sed "s/  - '+.//g" |
  sed "s/  - '/full:/g" |
  grep -v '#' |
  grep -v 'acg.rip' |
  sed "s/'//g" > "$DATA_DIR/cn"

copy_v2fly category-forums
copy_v2fly jetbrains
copy_v2fly jetbrains-ai
copy_v2fly category-ai-!cn
copy_v2fly category-ai-cn
copy_v2fly category-pt
copy_v2fly agilebits
copy_v2fly category-games-cn
copy_v2fly netease
copy_v2fly dlsite
copy_v2fly faceit

# steam@cn: mark entries of the steam list that also appear in SteamCN.
fetch \
  "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/SteamCN/SteamCN.list" \
  "$TEMP_DIR/steamcn.list"
sed -i.bak '/^\s*#/d' "$TEMP_DIR/steamcn.list"
sed -i.bak 's/DOMAIN,//g' "$TEMP_DIR/steamcn.list"
sed -i.bak 's/DOMAIN-SUFFIX,//g' "$TEMP_DIR/steamcn.list"
sed -i.bak 's/DOMAIN-KEYWORD,/keyword:/g' "$TEMP_DIR/steamcn.list"
rm -f "$TEMP_DIR/steamcn.list.bak"
while IFS= read -r line; do
  [ -n "$line" ] || continue
  grep -q "$line @cn" "$DATA_DIR/steam" || {
    sed -i.bak "/$line/ s/$/ @cn/" "$DATA_DIR/steam"
    rm -f "$DATA_DIR/steam.bak"
  }
done < "$TEMP_DIR/steamcn.list"

{
  echo 'include:google'
  echo 'include:github'
  echo 'include:netflix'
  echo 'ipleak.net'
  echo 'browserleaks.org'
} >> "$DATA_DIR/proxylite"
echo 'full:o33249.ingest.sentry.io' >> "$DATA_DIR/openai"
echo 'openai.com' >> "$DATA_DIR/openai"

export NO_SKIP=true
rm -rf "$LITE_OUTPUT_DIR"
mkdir -p "$LITE_OUTPUT_DIR"
(
  cd "$COMMUNITY_DIR"
  go run ./ \
    --datapath="$DATA_DIR" \
    --outputdir="$LITE_OUTPUT_DIR" \
    --outputname geosite-lite.dat \
    --exportlists="apple,applemusic,bilibili,cloudflare,google,github,microsoft,openai,pixiv,spotify,youtube,icloud,instagram,linkedin,steam,tiktok,twitter,epicgames"
)

for list in \
  apple applemusic bilibili cloudflare google github microsoft openai pixiv \
  spotify youtube icloud instagram linkedin steam tiktok twitter epicgames; do
  if ! grep -Eq ':[@][[:alnum:]_!-]+' "$LITE_OUTPUT_DIR/$list.txt"; then
    echo "error: geosite-lite list $list has no attributes" >&2
    exit 1
  fi
done

mv "$LITE_OUTPUT_DIR/geosite-lite.dat" "$COMMUNITY_DIR/geosite-lite.dat"
