# Creating Your Own Config Wizard Template

This guide is for **VPN providers and service administrators** who want to create a custom configuration template (`config_template.json`) that users can configure through the built-in wizard interface.

---

## What is the Config Wizard?

The Config Wizard is a visual interface that helps users create their `config.json` file without manually editing JSON. As a provider, you create a template file (`config_template.json`) that defines:

- Default settings (DNS, logging, routing rules)
- Which rules users can enable/disable via checkboxes
- How generated proxy outbounds are inserted
- Default proxy groups and selectors

Users simply:
1. Enter their subscription URL or direct proxy links
2. Choose which optional rules to enable
3. Click "Save" to generate their `config.json`

---

## Quick Start

### Step 1: Create the Template File

Create a file named `config_template.json` in the `bin/` folder (next to the application executable).

### Step 2: Use This Minimal Template

Copy this skeleton and customize it:

```jsonc
{
  /** @ParcerConfig
    {
      "ParserConfig": {
        "version": 2,
        "proxies": [
          {
            "source": "https://your-subscription-url-here"
          }
        ],
        "outbounds": [
          {
            "tag": "auto-proxy-out",
            "type": "urltest",
            "options": {
              "url": "https://cp.cloudflare.com/generate_204",
              "interval": "5m",
              "tolerance": 100
            },
            "outbounds": {
              "proxies": {}
            }
          },
          {
            "tag": "proxy-out",
            "type": "selector",
            "options": {
              "default": "auto-proxy-out"
            },
            "outbounds": {
              "addOutbounds": ["direct-out", "auto-proxy-out"],
              "proxies": {}
            }
          }
        ]
      }
    }
  */

  "log": {
    "level": "info",
    "timestamp": true
  },

  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "direct_dns",
        "server": "9.9.9.9",
        "server_port": 53
      }
    ],
    "final": "direct_dns"
  },

  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "interface_name": "singbox-tun0",
      "address": ["172.16.0.1/30"],
      "mtu": 1400,
      "auto_route": true,
      "strict_route": false,
      "stack": "system"
    }
  ],

  "outbounds": [
    /** @PARSER_OUTBOUNDS_BLOCK */
    { "type": "direct", "tag": "direct-out" }
  ],

  "route": {
    "rules": [
      { "ip_is_private": true, "outbound": "direct-out" },
      { "domain_suffix": ["local"], "outbound": "direct-out" },
      
      /** @SelectableRule
        @label Block Ads
        @description Soft-block ads by rejecting connections
        @default
        { "rule_set": "ads-all", "action": "reject" },
      */
      
      /** @SelectableRule
        @label Route Russian domains directly
        @description Send Russian domain traffic directly
        { "rule_set": "ru-domains", "outbound": "direct-out" },
      */
    ],
    "final": "proxy-out"
  },

  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "secret": "CHANGE_THIS_TO_YOUR_SECRET_TOKEN"
    }
  }
}
```

### Step 3: Customize for Your Service

1. **Update `@ParcerConfig`**: Replace `"source": "https://your-subscription-url-here"` with your actual subscription URL (or leave empty if users will enter their own)
2. **Adjust outbound groups**: Modify the `outbounds` array in `@ParcerConfig` to match your proxy group structure
3. **Add your rules**: Replace the example `@SelectableRule` blocks with your own routing rules
4. **Set default final**: Change `"final": "proxy-out"` to match your default proxy group tag

---

## Understanding the Special Markers

### 1. `@ParcerConfig` Block

**Purpose**: Defines the default subscription parser configuration.

**Location**: Must be a block comment `/** @ParcerConfig ... */` at the top of your template.

**What it contains**:
- `version`: Always use `2`
- `proxies`: Array with subscription URLs (users can override these in the wizard)
- `outbounds`: Default proxy groups (urltest, selector, etc.) that will be available to users

**Example**:
```jsonc
/** @ParcerConfig
  {
    "ParserConfig": {
      "version": 2,
      "proxies": [{ "source": "https://example.com/subscription" }],
      "outbounds": [
        {
          "tag": "auto-proxy-out",
          "type": "urltest",
          "options": { "url": "https://cp.cloudflare.com/generate_204", "interval": "5m" },
          "outbounds": { "proxies": {} }
        }
      ]
    }
  }
*/
```

**Important**: The wizard normalizes this to version 2 format automatically. Make sure all `tag` values match what you reference in your `route` section.

---

### 2. `@PARSER_OUTBOUNDS_BLOCK` Marker

**Purpose**: Tells the wizard where to insert generated proxy outbounds from the user's subscription.

**Location**: Must be inside the `outbounds` array as a comment.

**How it works**:
- The wizard parses the user's subscription URL or direct links
- It generates individual proxy outbounds (vless://, vmess://, etc.)
- These are inserted at the location of `@PARSER_OUTBOUNDS_BLOCK`
- Any outbounds **after** this marker are preserved (like `direct-out`)

**Example**:
```jsonc
"outbounds": [
  /** @PARSER_OUTBOUNDS_BLOCK */
  { "type": "direct", "tag": "direct-out" },
  { "type": "block", "tag": "block-out" }
]
```

**Result**: Generated proxies appear first, then `direct-out`, then `block-out`.

---

### 3. `@SelectableRule` Blocks

**Purpose**: Creates user-friendly checkboxes for optional routing rules.

**Location**: Must be inside the `route.rules` array as block comments.

**Structure**:
```jsonc
/** @SelectableRule
  @label Display Name
  @description Detailed explanation shown in tooltip
  @default
  { "rule_set": "example", "outbound": "proxy-out" },
*/
```

**Directives** (all optional):
- `@label Text` - The checkbox label (required for clarity)
- `@description Text` - Shown when user clicks "?" button
- `@default` - Rule is checked/enabled by default

**Rule format**: Can be a single rule object or an array of rules:
```jsonc
// Single rule
/** @SelectableRule
  @label Block ads
  { "rule_set": "ads-all", "action": "reject" },
*/

// Multiple rules (array)
/** @SelectableRule
  @label Gaming rules
  [
    { "rule_set": "games", "outbound": "direct-out" },
    { "port": [27000, 27001], "outbound": "direct-out" }
  ],
*/
```

**Outbound selection**: If your rule has an `outbound` field, the wizard automatically shows a dropdown so users can choose which proxy group to use. Available options come from:
- Tags defined in `@ParcerConfig` outbounds
- Tags from generated proxies
- Always available: `direct-out`, `reject`, `drop`

**Special case - reject rules**: If your rule has `"action": "reject"`, the wizard offers `reject` and `drop` options instead of proxy groups.

---

## Complete Example: Real-World Template

Here's a more complete example showing best practices:

```jsonc
{
  /** @ParcerConfig
    {
      "ParserConfig": {
        "version": 2,
        "proxies": [
          {
            "source": "https://your-vpn-service.com/api/subscription?token=USER_TOKEN"
          }
        ],
        "outbounds": [
          {
            "tag": "auto-proxy-out",
            "type": "urltest",
            "options": {
              "url": "https://cp.cloudflare.com/generate_204",
              "interval": "5m",
              "tolerance": 100,
              "interrupt_exist_connections": true
            },
            "outbounds": {
              "proxies": {}
            },
            "comment": "Auto-select fastest proxy"
          },
          {
            "tag": "proxy-out",
            "type": "selector",
            "options": {
              "interrupt_exist_connections": true,
              "default": "auto-proxy-out"
            },
            "outbounds": {
              "addOutbounds": ["direct-out", "auto-proxy-out"],
              "proxies": {}
            },
            "comment": "Main proxy selector"
          }
        ]
      }
    }
  */

  "log": {
    "level": "warn",
    "timestamp": true
  },

  "dns": {
    "servers": [
      {
        "type": "udp",
        "tag": "direct_dns",
        "server": "9.9.9.9",
        "server_port": 53
      },
      {
        "type": "https",
        "tag": "cloudflare_doh",
        "server": "1.1.1.1",
        "server_port": 443,
        "path": "/dns-query"
      }
    ],
    "rules": [
      { "server": "cloudflare_doh" }
    ],
    "final": "direct_dns"
  },

  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "interface_name": "singbox-tun0",
      "address": ["172.16.0.1/30"],
      "mtu": 1400,
      "auto_route": true,
      "strict_route": false,
      "stack": "system"
    }
  ],

  "outbounds": [
    /** @PARSER_OUTBOUNDS_BLOCK */
    { "type": "direct", "tag": "direct-out" },
    { "type": "block", "tag": "block-out" }
  ],

  "route": {
    "rule_set": [
      {
        "tag": "ads-all",
        "type": "remote",
        "format": "binary",
        "url": "https://raw.githubusercontent.com/v2fly/domain-list-community/release/geosite-category-ads-all.srs",
        "download_detour": "direct-out",
        "update_interval": "24h"
      },
      {
        "tag": "ru-domains",
        "type": "inline",
        "format": "domain_suffix",
        "rules": [
          { "domain_suffix": ["ru", "xn--p1ai"] }
        ]
      }
    ],
    "rules": [
      { "inbound": "tun-in", "action": "resolve", "strategy": "prefer_ipv4" },
      { "inbound": "tun-in", "action": "sniff", "timeout": "1s" },
      { "protocol": "dns", "action": "hijack-dns" },
      { "ip_is_private": true, "outbound": "direct-out" },
      { "domain_suffix": ["local"], "outbound": "direct-out" },
      
      /** @SelectableRule
        @label Block Ads
        @description Soft-block ads by rejecting connections (recommended)
        @default
        { "rule_set": "ads-all", "action": "reject" },
      */
      
      /** @SelectableRule
        @label Russian domains direct
        @description Route Russian domains directly (faster for local services)
        { "rule_set": "ru-domains", "outbound": "direct-out" },
      */
      
      /** @SelectableRule
        @label Gaming traffic direct
        @description Route gaming traffic directly for lower latency
        @default
        { "rule_set": "games", "outbound": "direct-out" },
      */
    ],
    "final": "proxy-out",
    "auto_detect_interface": true
  },

  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "secret": "CHANGE_THIS_TO_YOUR_SECRET_TOKEN"
    }
  }
}
```

---

## Best Practices

### 1. Always Include Static Outbounds

Keep at least `direct-out` after `@PARSER_OUTBOUNDS_BLOCK`. Users need it for local traffic and as a fallback option.

### 2. Use Clear Labels and Descriptions

Good labels help users understand what each rule does:
- вњ… `@label Block Ads` 
- вќЊ `@label Rule 1`

Good descriptions explain the impact:
- вњ… `@description Soft-block ads by rejecting connections instead of dropping packets`
- вќЊ `@description Blocks ads`

### 3. Set Sensible Defaults

Use `@default` for rules that most users should enable:
- Ad blocking
- Common geo-routing rules
- Performance optimizations

Don't use `@default` for:
- Experimental features
- Rules that might break common services
- Region-specific rules (unless you're targeting that region)

### 4. Match Tag Names

Ensure all `tag` values referenced in `route.rules` exist in:
- Your `@ParcerConfig` outbounds, OR
- Static outbounds after `@PARSER_OUTBOUNDS_BLOCK`, OR
- Will be generated from subscriptions

### 5. Validate Your JSON

After removing comments, your template must be valid JSON. Test it:
```bash
# Using jq (if installed)
cat config_template.json | jq . > /dev/null

# Or use an online JSON validator
```

### 6. Keep Section Order Logical

The wizard preserves your section order. Organize logically:
1. `log` - Logging settings
2. `dns` - DNS configuration
3. `inbounds` - Incoming connections
4. `outbounds` - Outgoing connections (with `@PARSER_OUTBOUNDS_BLOCK`)
5. `route` - Routing rules
6. `experimental` - Experimental features

---

## Testing Your Template

### Step 1: Place the File

Put `config_template.json` in the `bin/` folder next to the executable.

### Step 2: Launch the Wizard

1. Start the application
2. Open the Config Wizard (usually from the main menu or Tools tab)
3. Verify the template loads without errors

### Step 3: Test User Flow

1. **Tab 1 (Sources & Parser)**:
   - Enter a test subscription URL or direct link
   - Click "Check" - should validate successfully
   - Click "Parse" - should generate outbounds preview

2. **Tab 2 (Rules)**:
   - Verify all your `@SelectableRule` checkboxes appear
   - Check that outbound dropdowns show correct options
   - Toggle some rules on/off

3. **Tab 3 (Preview)**:
   - Click "Show Preview"
   - Verify the generated config looks correct
   - Check that selected rules are included
   - Verify `@PARSER_OUTBOUNDS_BLOCK` was replaced with generated proxies

4. **Save**:
   - Click "Save"
   - Verify `config.json` is created
   - Check that old config was backed up (if existed)
   - Verify `experimental.clash_api.secret` was generated

---

## Troubleshooting

### Template Not Loading

**Problem**: Wizard shows "Template file not found" or JSON errors.

**Solutions**:
- Ensure file is named exactly `config_template.json` (case-sensitive)
- Ensure file is in `bin/` folder
- Validate JSON syntax (remove comments and test)
- Check for trailing commas or syntax errors

### Generated Outbounds Not Appearing

**Problem**: After parsing, no outbounds show in preview.

**Solutions**:
- Verify `@PARSER_OUTBOUNDS_BLOCK` marker exists in `outbounds` array
- Check subscription URL is accessible
- Verify subscription format is supported (vless://, vmess://, trojan://, ss://)
- Check logs in `logs/` folder for parsing errors

### Rules Not Showing in Wizard

**Problem**: `@SelectableRule` blocks don't appear as checkboxes.

**Solutions**:
- Verify `@SelectableRule` is spelled correctly (case-sensitive)
- Ensure block is inside `route.rules` array
- Check that JSON inside block is valid
- Ensure `@label` directive is present

### Outbound Tags Not Found

**Problem**: Wizard shows "outbound not found" errors.

**Solutions**:
- Verify all referenced tags exist in `@ParcerConfig` outbounds
- Ensure `final` tag exists
- Check that generated proxies will have matching tags (if using tag filters)
- Add missing outbounds to static section after `@PARSER_OUTBOUNDS_BLOCK`

---

## Advanced Tips

### Dynamic Subscription URLs

If users need to enter their own subscription URL, leave `source` empty or use a placeholder:
```jsonc
"proxies": [{ "source": "" }]
```

Users will enter their URL in the wizard's first tab.

### Multiple Subscription Sources

Support multiple subscriptions:
```jsonc
"proxies": [
  { "source": "https://provider1.com/subscription" },
  { "source": "https://provider2.com/subscription" }
]
```

Users can add more in the wizard interface.

### Conditional Rules Based on Outbound Selection

When a rule has `outbound`, users can choose from available tags. Make sure your `@ParcerConfig` defines all options users might need.

### Custom Rule Sets

Reference remote rule sets in `route.rule_set`:
```jsonc
"rule_set": [
  {
    "tag": "ads-all",
    "type": "remote",
    "format": "binary",
    "url": "https://example.com/rules/ads.srs",
    "download_detour": "direct-out",
    "update_interval": "24h"
  }
]
```

Then reference them in `@SelectableRule` blocks.

---

## Distribution

When distributing your customized launcher:

1. Include `config_template.json` in your release package
2. Place it in `bin/` folder
3. Users will automatically use it when opening the Config Wizard
4. Consider documenting your specific rules and options in a separate README

---

## Need Help?

- **Template syntax issues**: Check this guide's examples
- **sing-box configuration**: See [official sing-box docs](https://sing-box.sagernet.org/configuration/)
- **ParserConfig format**: See `ParserConfig.md` in this repository
- **Report bugs**: Open an issue on [GitHub](https://github.com/Leadaxe/singbox-launcher/issues)

---

**Note**: This wizard template system is designed to make configuration easier for end users. As a provider, you maintain full control over the default configuration while giving users flexibility to customize their setup.



