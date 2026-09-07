# Geo Rules 数据说明

规则使用 [Loyalsoldier/v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat) 的 `geosite.dat`，以及 [Loyalsoldier/geoip](https://github.com/Loyalsoldier/geoip) 的 `geoip.dat` 和 `Country.mmdb` 作为上游数据；不使用其 Nginx 列表。

## geosite-lite 集合

`geosite-lite.dat` 从上游 `geosite.dat` 按本机 Mihomo 配置实际使用的分类提取生成。保留的分类由 [`geosite-lite.txt`](./geosite-lite.txt) 定义，并覆盖规则与 DNS `fake-ip-filter` 中引用的 geosite。

当前保留的分类包括：`private`、`oppo`、`agilebits`、`netease`、`category-ai-!cn`、`github`、`google`、`jetbrains`、`linuxdo`、`instagram`、`bahamut`、`linkedin`、`pixiv`、`dlsite`、`faceit`、`onedrive`、`microsoft`、`telegram`、`apple`、`category-games-cn`、`cn`。

提取过程会保留域名类型及 `@cn`、`@ads` 等 attribute。

## geoip-lite 集合

`geoip-lite.dat` 同样从上游 `geoip.dat` 提取，仅包含 `CN`、`JP`、`TW` 三个国家/地区分类。

## 输出结构

- `geo/`：上游原始 `geosite.dat`、`geoip.dat`、`Country.mmdb`，以及转换后的 `.db`、`.metadb` 和 lite 数据库版本。
- `clash/domain/`：完整 geosite 分类的 domain behavior `.yaml` / `.txt`，作为 Mihomo `.mrs` 的构建输入。
- `clash/ipcidr/`：完整 geoip 分类的 ipcidr behavior `.yaml` / `.txt`，作为 Mihomo `.mrs` 的构建输入。
- `mihomo/geosite/`：完整 geosite 分类的 classical behavior `.yaml` / `.txt`，以及从 `clash/domain/` 构建的 `.mrs`。
- `mihomo/geoip/`：完整 geoip 分类的 classical behavior `.yaml` / `.txt`，以及从 `clash/ipcidr/` 构建的 `.mrs`。
- `sing-box/geosite/` 和 `sing-box/geoip/`：完整分类的 `.srs` / `.json` ruleset。

规则集按分类拆分，Mihomo 和 sing-box 不再额外发布 lite 目录；需要精简整体数据库时使用 `geo/` 下的 lite 数据库文件。
