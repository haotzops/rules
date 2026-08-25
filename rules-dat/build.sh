#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="${GITHUB_WORKSPACE:-$(pwd)}"
RELEASE_WORK="${RELEASE_WORK:?RELEASE_WORK is required}"
CUSTOM_DIR="${CUSTOM_DIR:-$ROOT_DIR/custom}"
COMMUNITY_DIR="${COMMUNITY_DIR:-$ROOT_DIR/community}"
CONVERTER_DIR="${CONVERTER_DIR:-$ROOT_DIR/convert}"
GFWLIST_DIR="${GFWLIST_DIR:-$ROOT_DIR/gfwlist2dnsmasq}"
RESOURCES_DIR="$ROOT_DIR/rules-dat/resouces"
BUILD_DIR="${RUNNER_TEMP:-/tmp}/meta-rules-dat-build"
RULES_DAT_DIR="$RELEASE_WORK/rules-dat"
GEO_DIR="$RULES_DAT_DIR/geo"
MIHOMO_DIR="$RULES_DAT_DIR/mihomo"
SING_BOX_DIR="$RULES_DAT_DIR/sing-box"

CHINA_DOMAINS_URL="https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/ChinaMax/ChinaMax_Domain.txt"
GOOGLE_DOMAINS_URL="https://raw.githubusercontent.com/felixonmars/dnsmasq-china-list/master/google.china.conf"
APPLE_DOMAINS_URL="https://raw.githubusercontent.com/felixonmars/dnsmasq-china-list/master/apple.china.conf"
CUSTOM_PROXY_URL="https://raw.githubusercontent.com/Loyalsoldier/domain-list-custom/release/geolocation-!cn.txt"
WIN_SPY_URL="https://raw.githubusercontent.com/crazy-max/WindowsSpyBlocker/master/data/hosts/spy.txt"
WIN_UPDATE_URL="https://raw.githubusercontent.com/crazy-max/WindowsSpyBlocker/master/data/hosts/update.txt"
WIN_EXTRA_URL="https://raw.githubusercontent.com/crazy-max/WindowsSpyBlocker/master/data/hosts/extra.txt"

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

# Generate the source lists used by domain-list-community, following the upstream workflow.
chmod +x "$GFWLIST_DIR/gfwlist2dnsmasq.sh"
(
  cd "$GFWLIST_DIR"
  ./gfwlist2dnsmasq.sh -l -o "$BUILD_DIR/temp-gfwlist.txt"
)

fetch "$CHINA_DOMAINS_URL" "$BUILD_DIR/temp-direct.txt"
sed -i.bak '/^\s*#/d' "$BUILD_DIR/temp-direct.txt"
sed -i.bak '/^[^\.]/ s/^/full:/' "$BUILD_DIR/temp-direct.txt"
sed -i.bak 's/^\.\([^.]*\)/\1/' "$BUILD_DIR/temp-direct.txt"
rm -f "$BUILD_DIR/temp-direct.txt.bak"

perl -ne '/^((?=^.{3,255})[a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})+)/ && print "$1\n"' \
  "$BUILD_DIR/temp-gfwlist.txt" > "$BUILD_DIR/temp-proxy.txt"
fetch "$GOOGLE_DOMAINS_URL" "$BUILD_DIR/google.china.conf"
fetch "$APPLE_DOMAINS_URL" "$BUILD_DIR/apple.china.conf"
fetch "$CUSTOM_PROXY_URL" "$BUILD_DIR/custom-proxy.txt"
{
  perl -ne '/^server=\/([^\/]+)\// && print "$1\n"' "$BUILD_DIR/google.china.conf"
  perl -ne '/^server=\/([^\/]+)\// && print "$1\n"' "$BUILD_DIR/apple.china.conf"
} >> "$BUILD_DIR/temp-proxy.txt"
grep -Ev ':@cn' "$BUILD_DIR/custom-proxy.txt" |
  perl -ne '/^(domain):([^:]+)(\n$|:@.+)/ && print "$2\n"' >> "$BUILD_DIR/temp-proxy.txt"
grep -Ev ':@cn' "$BUILD_DIR/custom-proxy.txt" |
  perl -ne '/^((full|regexp|keyword):[^:]+)(\n$|:@.+)/ && print "$1\n"' |
  sort --ignore-case -u > "$BUILD_DIR/proxy-reserve.txt"

cat "$RESOURCES_DIR/proxy.txt" >> "$BUILD_DIR/temp-proxy.txt"
cat "$RESOURCES_DIR/direct.txt" >> "$BUILD_DIR/temp-direct.txt"

sort --ignore-case -u "$BUILD_DIR/temp-proxy.txt" > "$BUILD_DIR/proxy-list-with-redundant"
sort --ignore-case -u "$BUILD_DIR/temp-direct.txt" > "$BUILD_DIR/direct-list-with-redundant"

python3 "$RESOURCES_DIR/findRedundantDomain.py" \
  "$BUILD_DIR/direct-list-with-redundant" "$BUILD_DIR/direct-list-deleted-unsort"
python3 "$RESOURCES_DIR/findRedundantDomain.py" \
  "$BUILD_DIR/proxy-list-with-redundant" "$BUILD_DIR/proxy-list-deleted-unsort"
: > "$BUILD_DIR/direct-list-deleted-unsort.tmp"
: > "$BUILD_DIR/proxy-list-deleted-unsort.tmp"
test -f "$BUILD_DIR/direct-list-deleted-unsort" || mv "$BUILD_DIR/direct-list-deleted-unsort.tmp" "$BUILD_DIR/direct-list-deleted-unsort"
test -f "$BUILD_DIR/proxy-list-deleted-unsort" || mv "$BUILD_DIR/proxy-list-deleted-unsort.tmp" "$BUILD_DIR/proxy-list-deleted-unsort"
rm -f "$BUILD_DIR/direct-list-deleted-unsort.tmp" "$BUILD_DIR/proxy-list-deleted-unsort.tmp"
sort "$BUILD_DIR/direct-list-deleted-unsort" > "$BUILD_DIR/direct-list-deleted-sort"
sort "$BUILD_DIR/proxy-list-deleted-unsort" > "$BUILD_DIR/proxy-list-deleted-sort"

python3 "$RESOURCES_DIR/removeFrom.py" \
  -remove "$BUILD_DIR/direct-list-deleted-sort" \
  -from "$BUILD_DIR/direct-list-with-redundant" \
  -out "$BUILD_DIR/direct-list-without-redundant"
python3 "$RESOURCES_DIR/removeFrom.py" \
  -remove "$BUILD_DIR/proxy-list-deleted-sort" \
  -from "$BUILD_DIR/proxy-list-with-redundant" \
  -out "$BUILD_DIR/proxy-list-without-redundant"
python3 "$RESOURCES_DIR/removeFrom.py" \
  -remove "$RESOURCES_DIR/direct-need-to-remove.txt" \
  -from "$BUILD_DIR/direct-list-without-redundant" \
  -out "$BUILD_DIR/temp-cn.txt"
python3 "$RESOURCES_DIR/removeFrom.py" \
  -remove "$RESOURCES_DIR/proxy-need-to-remove.txt" \
  -from "$BUILD_DIR/proxy-list-without-redundant" \
  -out "$BUILD_DIR/temp-geolocation-!cn.txt"

mkdir -p "$COMMUNITY_DIR/data"
cat "$BUILD_DIR/temp-cn.txt" |
  grep -v google |
  grep -v manhua |
  grep -v ooklaserver |
  grep -v 'acg.rip' |
  sort --ignore-case -u |
  perl -ne '/^((?=^.{1,255})[a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})*)/ && print "$1\n"' > "$COMMUNITY_DIR/data/cn"
cat "$BUILD_DIR/temp-cn.txt" |
  sort --ignore-case -u |
  perl -ne 'print if not /^((?=^.{3,255})[a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})+)/' > "$BUILD_DIR/direct-tld-list.txt"
cat "$BUILD_DIR/temp-geolocation-!cn.txt" |
  sort --ignore-case -u |
  perl -ne '/^((?=^.{1,255})[a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})*)/ && print "$1\n"' |
  perl -ne 'print if not /\.cn$/' > "$COMMUNITY_DIR/data/geolocation-!cn"
cat "$BUILD_DIR/temp-geolocation-!cn.txt" |
  sort --ignore-case -u |
  perl -ne 'print if not /^((?=^.{3,255})[a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})+)/' > "$BUILD_DIR/proxy-tld-list.txt"

cat "$BUILD_DIR/direct-reserve.txt" 2>/dev/null >> "$COMMUNITY_DIR/data/cn" || true
cat "$BUILD_DIR/proxy-reserve.txt" 2>/dev/null >> "$COMMUNITY_DIR/data/geolocation-!cn" || true
cp "$COMMUNITY_DIR/data/cn" "$BUILD_DIR/direct-list.txt"
cp "$COMMUNITY_DIR/data/geolocation-!cn" "$BUILD_DIR/proxy-list.txt"

fetch "$GOOGLE_DOMAINS_URL" "$BUILD_DIR/google.china.conf.full"
fetch "$APPLE_DOMAINS_URL" "$BUILD_DIR/apple.china.conf.full"
perl -ne '/^server=\/([^\/]+)\// && print "full:$1\n"' "$BUILD_DIR/google.china.conf.full" > "$COMMUNITY_DIR/data/google-cn"
perl -ne '/^server=\/([^\/]+)\// && print "full:$1\n"' "$BUILD_DIR/apple.china.conf.full" > "$COMMUNITY_DIR/data/apple-cn"
cat "$BUILD_DIR/temp-gfwlist.txt" |
  perl -ne '/^((?=^.{3,255})[a-zA-Z0-9][-_a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-_a-zA-Z0-9]{0,62})+)/ && print "$1\n"' > "$COMMUNITY_DIR/data/gfw"
fetch "$WIN_SPY_URL" "$BUILD_DIR/spy.txt"
fetch "$WIN_UPDATE_URL" "$BUILD_DIR/update.txt"
fetch "$WIN_EXTRA_URL" "$BUILD_DIR/extra.txt"
grep '0.0.0.0' "$BUILD_DIR/spy.txt" | awk '{print $2}' > "$COMMUNITY_DIR/data/win-spy"
grep '0.0.0.0' "$BUILD_DIR/update.txt" | awk '{print $2}' > "$COMMUNITY_DIR/data/win-update"
grep '0.0.0.0' "$BUILD_DIR/extra.txt" | awk '{print $2}' > "$COMMUNITY_DIR/data/win-extra"

fetch \
  "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/SteamCN/SteamCN.list" \
  "$BUILD_DIR/steamcn.txt"
sed -i.bak '/^\s*#/d' "$BUILD_DIR/steamcn.txt"
sed -i.bak 's/DOMAIN,//g' "$BUILD_DIR/steamcn.txt"
sed -i.bak 's/DOMAIN-SUFFIX,//g' "$BUILD_DIR/steamcn.txt"
sed -i.bak 's/DOMAIN-KEYWORD,/keyword:/g' "$BUILD_DIR/steamcn.txt"
rm -f "$BUILD_DIR/steamcn.txt.bak"
while IFS= read -r line; do
  grep -q "$line @cn" "$COMMUNITY_DIR/data/steam" 2>/dev/null ||
    sed -i "/$line/ s/$/ @cn/" "$COMMUNITY_DIR/data/steam"
done < "$BUILD_DIR/steamcn.txt"

fetch "https://raw.githubusercontent.com/xishang0128/rules/main/biliintl.list" "$COMMUNITY_DIR/data/biliintl"
fetch "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/OneDrive/OneDrive.list" "$BUILD_DIR/onedrive.list"
sed -i.bak '/^\s*#/d' "$BUILD_DIR/onedrive.list"
sed -i.bak 's/^PROCESS-NAME,//' "$BUILD_DIR/onedrive.list"
sed -i.bak 's/DOMAIN,/full:/g' "$BUILD_DIR/onedrive.list"
sed -i.bak 's/DOMAIN-SUFFIX,//g' "$BUILD_DIR/onedrive.list"
sed -i.bak 's/DOMAIN-KEYWORD,/keyword:/g' "$BUILD_DIR/onedrive.list"
sed -i.bak '/^\s*IP-CIDR/d' "$BUILD_DIR/onedrive.list"
rm -f "$BUILD_DIR/onedrive.list.bak"
cp "$BUILD_DIR/onedrive.list" "$COMMUNITY_DIR/data/onedrive"
echo 'sharepoint.cn' >> "$COMMUNITY_DIR/data/onedrive"
fetch "https://raw.githubusercontent.com/xishang0128/rules/main/sharepoint.list" "$COMMUNITY_DIR/data/sharepoint"
fetch "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/PrivateTracker/PrivateTracker.list" "$BUILD_DIR/tracker.list"
sed -i.bak '/^\s*#/d' "$BUILD_DIR/tracker.list"
sed -i.bak 's/^PROCESS-NAME,//' "$BUILD_DIR/tracker.list"
sed -i.bak 's/DOMAIN,/full:/g' "$BUILD_DIR/tracker.list"
sed -i.bak 's/DOMAIN-SUFFIX,//g' "$BUILD_DIR/tracker.list"
sed -i.bak 's/DOMAIN-KEYWORD,/keyword:/g' "$BUILD_DIR/tracker.list"
sed -i.bak '/^\s*IP-CIDR/d' "$BUILD_DIR/tracker.list"
rm -f "$BUILD_DIR/tracker.list.bak"
cp "$BUILD_DIR/tracker.list" "$COMMUNITY_DIR/data/tracker"
fetch "https://raw.githubusercontent.com/XIU2/TrackersListCollection/master/all.txt" "$BUILD_DIR/all-trackers.txt"
grep -i '\.[A-Z]' "$BUILD_DIR/all-trackers.txt" |
  grep -v tracker |
  sed 's/^.*\/\///g' |
  sed 's/:.*\/.*//g' >> "$COMMUNITY_DIR/data/tracker"
fetch "https://github.com/blackmatrix7/ios_rule_script/raw/master/rule/Clash/BlockHttpDNS/BlockHttpDNS.list" "$BUILD_DIR/httpdns.list"
sed -i.bak '/^\s*#/d' "$BUILD_DIR/httpdns.list"
sed -i.bak 's/^PROCESS-NAME,//' "$BUILD_DIR/httpdns.list"
sed -i.bak 's/DOMAIN,/full:/g' "$BUILD_DIR/httpdns.list"
sed -i.bak 's/DOMAIN-SUFFIX,//g' "$BUILD_DIR/httpdns.list"
sed -i.bak 's/DOMAIN-KEYWORD,/keyword:/g' "$BUILD_DIR/httpdns.list"
sed -i.bak '/^\s*IP-CIDR/d' "$BUILD_DIR/httpdns.list"
rm -f "$BUILD_DIR/httpdns.list.bak"
cp "$BUILD_DIR/httpdns.list" "$COMMUNITY_DIR/data/httpdns"
echo 'zed.dev' > "$COMMUNITY_DIR/data/category-dev"

(
  cd "$CUSTOM_DIR"
  echo 'ipleak.net' >> "$COMMUNITY_DIR/data/geolocation-!cn"
  echo 'browserleaks.org' >> "$COMMUNITY_DIR/data/geolocation-!cn"
  go run ./ --datapath="$COMMUNITY_DIR/data"
)

COMMUNITY_DIR="$COMMUNITY_DIR" "$ROOT_DIR/rules-dat/build-geosite-lite.sh"

# Download the GeoIP and ASN source data used by the upstream workflow.
fetch "https://github.com/xishang0128/geoip/raw/release/geoip.dat" "$BUILD_DIR/geoip-lite.dat"
fetch "https://github.com/Loyalsoldier/geoip/raw/release/geoip.dat" "$BUILD_DIR/geoip.dat"
fetch "https://raw.githubusercontent.com/xishang0128/geoip/release/Country.mmdb" "$BUILD_DIR/country-lite.mmdb"
fetch "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/Country.mmdb" "$BUILD_DIR/country.mmdb"
fetch "https://raw.githubusercontent.com/xishang0128/geoip/release/GeoLite2-ASN.mmdb" "$BUILD_DIR/GeoLite2-ASN.mmdb"

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
  "$BUILD_DIR/sing-rule/geo-lite/geoip" \
  "$BUILD_DIR/meta-rule/asn" "$BUILD_DIR/sing-rule/asn"

"$GEO_BIN" convert site -i v2ray -o sing -f "$BUILD_DIR/geosite.db" "$CUSTOM_DIR/publish/geosite.dat"
"$GEO_BIN" convert site -i v2ray -o sing -f "$BUILD_DIR/geosite-lite.db" "$COMMUNITY_DIR/geosite-lite.dat"
"$GEO_BIN" convert ip -i v2ray -o sing -f "$BUILD_DIR/geoip.db" "$BUILD_DIR/geoip.dat"
"$GEO_BIN" convert ip -i v2ray -o meta -f "$BUILD_DIR/geoip.metadb" "$BUILD_DIR/geoip.dat"
"$GEO_BIN" convert ip -i v2ray -o sing -f "$BUILD_DIR/geoip-lite.db" "$BUILD_DIR/geoip-lite.dat"
"$GEO_BIN" convert ip -i v2ray -o meta -f "$BUILD_DIR/geoip-lite.metadb" "$BUILD_DIR/geoip-lite.dat"

(
  cd "$CONVERTER_DIR"
  go run ./ geosite -f "$CUSTOM_DIR/publish/geosite.dat" -o "$BUILD_DIR/sing-rule/geo/geosite" -t sing-box
  go run ./ geoip -f "$BUILD_DIR/geoip.dat" -o "$BUILD_DIR/sing-rule/geo/geoip" -t sing-box
  go run ./ geosite -f "$COMMUNITY_DIR/geosite-lite.dat" -o "$BUILD_DIR/sing-rule/geo-lite/geosite" -t sing-box
  go run ./ geoip -f "$BUILD_DIR/geoip-lite.dat" -o "$BUILD_DIR/sing-rule/geo-lite/geoip" -t sing-box
  go run ./ geosite -f "$CUSTOM_DIR/publish/geosite.dat" -o "$BUILD_DIR/meta-rule/geo/geosite"
  go run ./ geoip -f "$BUILD_DIR/geoip.dat" -o "$BUILD_DIR/meta-rule/geo/geoip"
  go run ./ geosite -f "$COMMUNITY_DIR/geosite-lite.dat" -o "$BUILD_DIR/meta-rule/geo-lite/geosite"
  go run ./ geoip -f "$BUILD_DIR/geoip-lite.dat" -o "$BUILD_DIR/meta-rule/geo-lite/geoip"
  go run ./ asn -f "$BUILD_DIR/GeoLite2-ASN.mmdb" -o "$BUILD_DIR/meta-rule/asn"
  go run ./ asn -f "$BUILD_DIR/GeoLite2-ASN.mmdb" -o "$BUILD_DIR/sing-rule/asn" -t sing-box
)

(
  cd "$BUILD_DIR/meta-rule"
  7z a "$GEO_DIR/BundleMRS.7z" -mx=1 -r '*.mrs'
)

cp "$BUILD_DIR/country.mmdb" "$GEO_DIR/"
cp "$BUILD_DIR/country-lite.mmdb" "$GEO_DIR/"
cp "$BUILD_DIR/GeoLite2-ASN.mmdb" "$GEO_DIR/"
cp "$BUILD_DIR/geoip.dat" "$GEO_DIR/"
cp "$BUILD_DIR/geoip-lite.dat" "$GEO_DIR/"
cp "$BUILD_DIR/geoip.db" "$GEO_DIR/"
cp "$BUILD_DIR/geoip-lite.db" "$GEO_DIR/"
cp "$BUILD_DIR/geoip.metadb" "$GEO_DIR/"
cp "$BUILD_DIR/geoip-lite.metadb" "$GEO_DIR/"
cp "$CUSTOM_DIR/publish/geosite.dat" "$GEO_DIR/"
cp "$COMMUNITY_DIR/geosite-lite.dat" "$GEO_DIR/"
cp "$BUILD_DIR/geosite.db" "$GEO_DIR/"
cp "$BUILD_DIR/geosite-lite.db" "$GEO_DIR/"
cp -a "$BUILD_DIR/meta-rule/." "$MIHOMO_DIR/"
cp -a "$BUILD_DIR/sing-rule/." "$SING_BOX_DIR/"
cp "$ROOT_DIR/rules-dat/README_base.md" "$RULES_DAT_DIR/README_base.md"
cp "$ROOT_DIR/rules-dat/README.md" "$RULES_DAT_DIR/README.md"

while IFS= read -r -d '' file; do
  sha256sum "$file" > "$file.sha256sum"
done < <(find "$GEO_DIR" -type f ! -name '*.sha256sum' -print0)

printf 'built MetaCubeX Geo data and rulesets in %s\n' "$RULES_DAT_DIR"
