# Release Notes — v0.8.0

> Full details: [docs/release_notes/0-8-0.md](docs/release_notes/0-8-0.md)

<details>
<summary><b>🇬🇧 English</b></summary>

### Highlights
- **Get free VPN!** — one-click button downloads ready-made config from GitHub and fills in Sources + ParserConfig.  
- **Wizard state save/load** — save, switch, and restore multiple configurations (`state.json`)
- **New rule types** — *Processes* (select running apps) and *Custom JSON* (arbitrary rule body)
- **Auto-parse on Rules tab** — outbounds always up-to-date when you switch tabs
- **Wizard single-instance** + click-redirect overlay + tray "Open" focus
- **Chat GPT button** — builds a prompt from ParserConfig and opens ChatGPT
- **Per-selector last proxy** — remembers last proxy per group, shows live active outbound
- **Hide app from Dock** (macOS) — [PR #23](https://github.com/Leadaxe/singbox-launcher/pull/23) by [@MustDie-green](https://github.com/MustDie-green)
- **Unified config template** — single `config_template.json` for all platforms, JSON structure with `params` for platform-specific settings, no more comment-based directives
- **Logging centralization** — unified `debuglog.*` API, env-controlled log level
- **Singleton Controller** — simplified `core.*` API, removed parameter passing
- **CI/CD** — Go build cache, golangci-lint v2.8.0, faster builds

</details>

<details>
<summary><b>🇷🇺 Русская версия</b></summary>

### Основное
- **Get free VPN!** — кнопка в один клик скачивает готовый конфиг с GitHub и заполняет Sources + ParserConfig.  
- **Сохранение/загрузка состояний визарда** — сохраняйте, переключайте и восстанавливайте несколько конфигураций (`state.json`)
- **Новые типы правил** — *Processes* (выбор запущенных приложений) и *Custom JSON* (произвольное тело правила)
- **Автопарсинг на вкладке Rules** — outbounds всегда актуальны при переключении вкладок
- **Визард в одном экземпляре** + оверлей перенаправления кликов + фокус через "Open" в трее
- **Кнопка Chat GPT** — формирует промпт из ParserConfig и открывает ChatGPT
- **Per-selector last proxy** — запоминает последний прокси для каждой группы, показывает активный outbound в реальном времени
- **Скрытие из Dock** (macOS) — [PR #23](https://github.com/Leadaxe/singbox-launcher/pull/23) от [@MustDie-green](https://github.com/MustDie-green)
- **Единый шаблон конфигурации** — один `config_template.json` для всех платформ, JSON-структура с `params` для платформо-зависимых настроек, без директив в комментариях
- **Централизация логирования** — единый API `debuglog.*`, управление уровнем через переменную окружения
- **Singleton Controller** — упрощён API `core.*`, убрана передача параметров
- **CI/CD** — кэш сборки Go, golangci-lint v2.8.0, ускорение билдов

</details>
