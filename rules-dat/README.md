# Geo Rules 数据说明

基于 `MetaCubeX/meta-rules-dat` 的 Geo 数据和 ruleset 构建方法。本项目增删了 geosite-lite 集合

## geosite-lite 集合

集合包含：

- **blackmatrix7 规则转换**：abema、apple、applemusic、bilibili、bahamut、cloudflare、google、github、microsoft、netflix、openai、onedrive、pixiv、proxy、spotify、telegram、twitter、tiktok、youtube、proxymedia、icloud、instagram、linkedin、steam（含 `steam@cn` 国区标记）、epicgames
- **v2fly 集合复制**：category-forums、jetbrains、jetbrains-ai、category-ai-!cn、category-ai-cn、category-pt、agilebits、category-games-cn、netease、dlsite、faceit
- **属性叠加**：从 v2fly 同名集合补充 `@cn`、`@!cn`、`@ads` 等原生 attribute，不覆盖 BlackMatrix7 的域名集合
- **其他来源**：biliintl、ehentai、private、cn

