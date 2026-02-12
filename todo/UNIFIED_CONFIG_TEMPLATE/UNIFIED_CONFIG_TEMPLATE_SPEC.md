# ТЗ: Единый config_template.json — структура

## Проблема

Два файла (`config_template.json`, `config_template_macos.json`) дублируют 95% кода.
Система директив в комментариях (`@SelectableRule`, `@ParserConfig`, `@PARSER_OUTBOUNDS_BLOCK`) — хрупкая, парсится регулярками.

## Идея

Один файл с явной JSON-структурой (по аналогии с `state.json`):
- Правила — отдельный массив
- Платформенные различия — через `params` с `platforms`
- Никаких директив в комментариях

## Полный пример единого config_template.json

```json
{
  "parser_config": {
    "ParserConfig": {
      "version": 4,
      "proxies": [{ "source": "https://your-subscription-url-here" }],
      "outbounds": [
        {
          "tag": "auto-proxy-out",
          "wizard": { "required": 1 },
          "type": "urltest",
          "options": {
            "url": "https://cp.cloudflare.com/generate_204",
            "interval": "5m",
            "tolerance": 100,
            "interrupt_exist_connections": true
          },
          "filters": { "tag": "!/(🇷🇺)/i" },
          "comment": "Proxy automated group for everything that should go through VPN"
        },
        {
          "tag": "proxy-out",
          "wizard": { "required": 1 },
          "type": "selector",
          "options": { "interrupt_exist_connections": true, "default": "auto-proxy-out" },
          "filters": { "tag": "!/(🇷🇺)/i" },
          "addOutbounds": ["direct-out", "auto-proxy-out"],
          "comment": "Proxy group for everything that should go through VPN"
        },
        {
          "tag": "ru VPN 🇷🇺",
          "type": "selector",
          "options": {
            "default": "direct-out",
            "interrupt_exist_connections": true
          },
          "filters": { "tag": "/(🇷🇺)/i" },
          "addOutbounds": ["direct-out"],
          "comment": "Proxy group for russian VPN"
        },
        {
          "tag": "vpn ①",
          "type": "selector",
          "options": {
            "default": "proxy-out",
            "interrupt_exist_connections": true
          },
          "addOutbounds": ["direct-out", "proxy-out"],
          "comment": "Proxy group 1"
        },
        {
          "tag": "vpn ②",
          "type": "selector",
          "options": {
            "default": "proxy-out",
            "interrupt_exist_connections": true
          },
          "addOutbounds": ["direct-out", "proxy-out"],
          "comment": "Proxy group 2"
        },
        {
          "tag": "go-any-way-githubusercontent",
          "wizard": { "hide": true, "required": 2 },
          "type": "urltest",
          "options": {
            "url": "https://raw.githubusercontent.com/github/gitignore/main/Global/GPG.gitignore",
            "interval": "1h",
            "idle_timeout": "1h",
            "tolerance": 1000,
            "interrupt_exist_connections": true
          },
          "addOutbounds": ["direct-out"],
          "comment": "find any way"
        }
      ]
    }
  },

  "config": {
    "log": {
      "level": "warn",
      "timestamp": true
    },
    "dns": {
      "servers": [
        {
          "type": "udp",
          "tag": "direct_dns_resolver",
          "server": "1.1.1.1",
          "server_port": 53
        },
        {
          "type": "https",
          "tag": "google_doh",
          "server": "8.8.8.8",
          "server_port": 443,
          "path": "/dns-query"
        },
        {
          "type": "https",
          "tag": "google_doh_vpn",
          "server": "8.8.8.8",
          "server_port": 443,
          "path": "/dns-query",
          "detour": "proxy-out"
        },
        {
          "type": "https",
          "tag": "yandex_doh",
          "server": "77.88.8.88",
          "server_port": 443,
          "path": "/dns-query",
          "domain_strategy": "prefer_ipv4"
        },
        {
          "type": "udp",
          "tag": "yandex_dns_vpn",
          "server": "77.88.8.2",
          "server_port": 53,
          "detour": "proxy-out",
          "domain_strategy": "prefer_ipv4"
        },
        {
          "type": "udp",
          "tag": "yandex_dns_direct",
          "server": "77.88.8.2",
          "server_port": 53,
          "domain_strategy": "prefer_ipv4"
        }
      ],
      "rules": [
        { "domain_suffix": ["githubusercontent.com", "github.com"], "server": "direct_dns_resolver" },
        { "rule_set": "ru-domains", "server": "yandex_doh" },
        { "rule_set": "ru-domains", "server": "yandex_dns_vpn" },
        { "rule_set": "ru-domains", "server": "yandex_dns_direct" },
        { "server": "google_doh" },
        { "server": "google_doh_vpn" }
      ],
      "final": "direct_dns_resolver",
      "strategy": "ipv4_only",
      "independent_cache": false
    },
    "inbounds": [],
    "outbounds": [
      { "type": "direct", "tag": "direct-out" }
    ],
    "route": {
      "default_domain_resolver": "direct_dns_resolver",
      "rule_set": [
        { "tag": "ru-domains", "type": "inline", "format": "domain_suffix", "rules": [{ "domain_suffix": ["ru", "xn--p1ai", "su"] }] }
      ],
      "rules": [
        { "protocol": "dns", "action": "hijack-dns" },
        { "ip_is_private": true, "outbound": "direct-out" },
        { "domain_suffix": ["local", "lan"], "outbound": "direct-out" }
      ],
      "final": "proxy-out",
      "auto_detect_interface": true
    },
    "experimental": {
      "clash_api": {
        "external_controller": "127.0.0.1:9090",
        "secret": "CHANGE_THIS_TO_YOUR_SECRET_TOKEN"
      },
      "cache_file": {
        "enabled": true,
        "path": "cache.db"
      }
    }
  },

  "selectable_rules": [
    {
      "label": "Block Ads (ads-all, soft)",
      "description": "Soft-block ads by rejecting connections instead of dropping packets",
      "default": true,
      "rule_set": [
        { "tag": "ads-all", "type": "remote", "format": "binary", "url": "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/sing-box/rule-set-geosite/geosite-category-ads-all.srs", "download_detour": "go-any-way-githubusercontent", "update_interval": "24h" }
      ],
      "rule": { "rule_set": "ads-all", "action": "reject" }
    },
    {
      "label": "Russian domains direct",
      "description": "Route Russian domains directly.",
      "rule": { "rule_set": "ru-domains", "outbound": "direct-out" }
    },
    {
      "label": "Russia-only services",
      "description": "Use Russian VPN group for services available only inside Russia.",
      "rule_set": [
        { "tag": "ru-inside", "type": "remote", "format": "binary", "url": "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/sing-box/rule-set-geosite/geosite-ru-available-only-inside.srs", "download_detour": "go-any-way-githubusercontent", "update_interval": "24h" }
      ],
      "rule": { "rule_set": "ru-inside", "outbound": "direct-out" }
    },
    {
      "label": "Russian blocked resources",
      "description": "Detour resources blocked in Russia through proxy selector.",
      "rule_set": [
        { "tag": "ru-blocked-main", "type": "remote", "format": "binary", "url": "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/sing-box/rule-set-geoip/geoip-ru-blocked.srs", "download_detour": "go-any-way-githubusercontent", "update_interval": "24h" },
        { "tag": "ru-blocked-community", "type": "remote", "format": "binary", "url": "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/sing-box/rule-set-geoip/geoip-ru-blocked-community.srs", "download_detour": "go-any-way-githubusercontent", "update_interval": "24h" }
      ],
      "rule": { "rule_set": ["ru-blocked-main", "ru-blocked-community"], "network": ["tcp", "udp"], "outbound": "proxy-out" }
    },
    {
      "label": "Israel-specific sites",
      "description": "Send Israel-focused traffic via main proxy selector.",
      "rule_set": [
        { "tag": "israel-sites", "type": "remote", "format": "binary", "url": "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/sing-box/rule-set-geoip/geoip-il.srs", "download_detour": "go-any-way-githubusercontent", "update_interval": "24h" }
      ],
      "rule": { "rule_set": "israel-sites", "outbound": "proxy-out" }
    },
    {
      "label": "Drop RU DNS over UDP",
      "description": "Block UDP DNS queries that target Russian domains.",
      "rule": { "rule_set": "ru-domains", "action": "reject", "method": "drop", "network": ["udp"], "port": [53] }
    },
    {
      "label": "BitTorrent direct",
      "description": "Route BitTorrent traffic directly to avoid VPN throttling.",
      "default": true,
      "rule": { "protocol": ["bittorrent"], "outbound": "direct-out" }
    },
    {
      "label": "2ip.io for VPN IP test",
      "description": "Direct route for 2ip.io to check VPN IP.",
      "default": true,
      "rule": { "domain": ["2ip.io"], "outbound": "proxy-out" }
    },
    {
      "label": "2ip.me via proxy",
      "description": "Route 2ip.me through proxy to verify VPN IP.",
      "rule": { "domain": ["2ip.me"], "outbound": "proxy-out" }
    },
    {
      "label": "2ip.ru your direct ip test",
      "description": "Direct route for 2ip.ru to check your IP.",
      "default": true,
      "rule": { "domain": ["2ip.ru"], "outbound": "direct-out" }
    },
    {
      "label": "Gemini via Gemini VPN",
      "description": "Use dedicated Gemini VPN selector for Gemini rule set.",
      "rule_set": [
        { "tag": "gemini", "type": "inline", "format": "domain_suffix", "rules": [{ "domain_suffix": ["generativelanguage.googleapis.com", "gemini.google.com", "ai.google.dev", "palm.googleapis.com"] }] }
      ],
      "rule": { "rule_set": "gemini", "network": ["tcp", "udp"], "outbound": "proxy-out" }
    },
    {
      "label": "Messengers via proxy",
      "description": "Send messenger traffic through proxy selector.",
      "default": true,
      "rule_set": [
        { "tag": "messengers", "type": "inline", "format": "domain_suffix", "rules": [{ "domain_suffix": ["meet.google.com", "googlevideo.com", "gstatic.com", "googleusercontent.com", "googleapis.com", "web.whatsapp.com", "whatsapp.com", "whatsapp.net", "cdn.whatsapp.net", "mmg.whatsapp.net", "discord.com", "discordapp.com", "cdn.discordapp.com", "discord.gg", "gateway.discord.gg", "discord.media", "telegram.org", "t.me", "cdn-telegram.org"] }, { "process_name": ["Telegram.exe", "Discord.exe", "WhatsApp.exe", "Signal.exe", "Zoom.exe"] }] }
      ],
      "rule": { "rule_set": "messengers", "network": ["tcp", "udp"], "outbound": "proxy-out" }
    },
    {
      "label": "Games direct",
      "description": "Send gaming rule set traffic directly for lower latency.",
      "default": true,
      "rule_set": [
        { "tag": "games", "type": "remote", "format": "binary", "url": "https://raw.githubusercontent.com/runetfreedom/russia-v2ray-rules-dat/release/sing-box/rule-set-geosite/geosite-category-games.srs", "download_detour": "go-any-way-githubusercontent", "update_interval": "24h" }
      ],
      "rule": { "rule_set": "games", "network": ["tcp", "udp"], "outbound": "direct-out" }
    },
    {
      "label": "Gaming ports direct",
      "description": "Keep popular gaming ports and Steam ranges outside VPN.",
      "default": true,
      "rule": {
        "port": [3659, 1935, 5001, 5795, 5796, 7000, 7777, 9000, 10039, 10040],
        "port_range": "27000:27100",
        "network": ["tcp", "udp"],
        "outbound": "direct-out"
      }
    },
    {
      "label": "stop HTTP/3",
      "description": "QUIC does not support UDP traffic proxying. Therefore, we stop HTTP/3. This may block QUIC traffic (used by Google, YouTube, Cloudflare, etc.), which may potentially reduce speed or availability for these services.",
      "rule": { "action": "reject", "method": "drop", "network": ["udp"], "port": [443] }
    }
  ],

  "params": [
    {
      "name": "inbounds",
      "platforms": ["win", "linux"],
      "value": [
        {
          "type": "tun",
          "tag": "tun-in",
          "interface_name": "singbox-tun0",
          "address": ["172.16.0.1/30"],
          "mtu": 1492,
          "auto_route": true,
          "strict_route": false,
          "route_exclude_address": [
            "10.0.0.0/8",
            "172.16.0.0/12",
            "192.168.0.0/16",
            "127.0.0.0/8",
            "169.254.0.0/16",
            "100.64.0.0/10",
            "224.0.0.0/4",
            "255.255.255.255/32",
            "198.18.0.0/15",
            "fc00::/7",
            "fe80::/10",
            "ff00::/8",
            "::1/128"
          ],
          "stack": "system"
        }
      ]
    },
    {
      "name": "route.rules",
      "platforms": ["win", "linux"],
      "mode": "prepend",
      "value": [
        { "inbound": "tun-in", "action": "resolve", "strategy": "prefer_ipv4" },
        { "inbound": "tun-in", "action": "sniff", "timeout": "1s" }
      ]
    },
    {
      "name": "inbounds",
      "platforms": ["darwin"],
      "value": [
        {
          "type": "mixed",
          "tag": "proxy-in",
          "listen": "127.0.0.1",
          "listen_port": 7890,
          "sniff": true,
          "sniff_override_destination": true,
          "set_system_proxy": true
        }
      ]
    }
  ]
}
```

## Описание секций

### `parser_config`
Конфигурация парсера — подписки, outbound-группы, настройки обновлений.
Без изменений от текущей схемы.

### `config`
Основной конфиг sing-box. Содержит все секции (log, dns, outbounds, route, experimental).
- `inbounds` — пустой массив `[]`, заполняется из `params` по платформе
- `outbounds` — содержит только статические элементы (`direct-out`), сгенерированные outbound-группы вставляются парсером в начало массива
- `route.rules` — только базовые универсальные правила (hijack-dns, ip_is_private, local); TUN-правила добавляются через `params`
- `route.rule_set` — только общие rule_set, используемые несколькими правилами или DNS (например `ru-domains`); остальные rule_set привязаны к конкретным `selectable_rules`
- Selectable rules больше не внутри `route.rules` — они в отдельной секции

### `selectable_rules`
Массив правил для визарда. Каждое правило:

| Поле | Тип | Обязательно | Описание |
|------|-----|-------------|----------|
| `label` | string | да | Название для отображения в визарде |
| `description` | string | да | Описание (tooltip) |
| `default` | bool | нет | Включено по умолчанию (default=false) |
| `rule_set` | array | нет | Определения rule_set, необходимые для этого правила. Добавляются в `config.route.rule_set` только если правило включено |
| `rule` | object | одно из двух | Одиночное правило маршрутизации |
| `rules` | array | одно из двух | Несколько правил (если правило состоит из нескольких JSON-объектов) |

**`rule` vs `rules`**: используется одно из двух. `rule` — один объект, `rules` — массив (для правил из нескольких JSON-объектов).

**`rule_set`**: если правило ссылается на rule_set (например `"rule_set": "ads-all"`), определение этого rule_set хранится здесь же.
Правило и его зависимости — единое целое. Включил правило → rule_set добавился в конфиг, выключил → не попадает.
Правила без `rule_set` (например, по `domain`, `port`, `protocol`) просто не имеют этого поля.
Общие rule_set, используемые несколькими правилами или DNS (например `ru-domains`), остаются в `config.route.rule_set`.

Платформозависимые правила (TUN resolve+sniff) НЕ являются selectable — они в `params` с `mode: "prepend"`.

### `params`
Массив платформенных параметров. Каждый элемент:

| Поле | Тип | Обязательно | Описание |
|------|-----|-------------|----------|
| `name` | string | да | Путь к секции в `config` (точечная нотация: `"inbounds"`, `"route.rules"`) |
| `platforms` | array | да | Платформы: `"win"`, `"linux"`, `"darwin"` |
| `mode` | string | нет | `"replace"` (по умолчанию), `"prepend"` (вставить в начало массива) или `"append"` (добавить в конец массива) |
| `value` | any | да | Значение для подстановки в `config[name]` |

**Логика применения:**
1. При загрузке шаблона определяется текущая ОС
2. Для каждого `param` проверяется `platforms`
3. Если текущая ОС в списке:
   - `mode: "replace"` (default) → `config[name]` заменяется на `value`
   - `mode: "prepend"` → элементы `value` вставляются в начало массива `config[name]`
   - `mode: "append"` → элементы `value` добавляются в конец массива `config[name]`
4. Если для секции нет подходящего `param` → остаётся значение из `config`

**Маппинг платформ:**
- `"win"` → `runtime.GOOS == "windows"`
- `"linux"` → `runtime.GOOS == "linux"`
- `"darwin"` → `runtime.GOOS == "darwin"`

## process_name

`cloneRule()` уже нормализует `.exe` суффиксы при генерации.
В шаблоне хранятся Windows-имена (`Telegram.exe`), на macOS/Linux `.exe` удаляется автоматически.
Для macOS-специфичных имён (например, `zoom.us` vs `Zoom.exe`) — маппинг в коде.

## Что удаляется

- `bin/config_template_macos.json`
- `GetTemplateFileName()` / `GetTemplateURL()` — упрощаются (один файл)
- Все regex-парсеры: `extractCommentBlock()`, `extractAllSelectableBlocks()`, `parseSelectableRules()`, `extractRuleMetadata()`, `normalizeRuleJSON()`
- Директивы `@ParserConfig`, `@SelectableRule`, `@PARSER_OUTBOUNDS_BLOCK` — заменяются структурой JSON

## Что остаётся

- `cloneRule()` + `normalizeProcessNames()` — нормализация process_name
- `MergeRouteSection()` — объединение базовых + selectable + custom rules
- `BuildParserOutboundsBlock()` — генерация outbound-блока из подписок

## Совместимость с state.json

`state.json` сохраняет `selectable_rule_states` и `custom_rules` — это пользовательский выбор.
Шаблон (`config_template.json`) определяет доступные правила и структуру конфига.
Связь: `state.json.selectable_rule_states[i]` ↔ `template.selectable_rules[i]` — по индексу или по `label`.
