# Upcoming release — черновик

Сюда складываем пункты, которые войдут в следующий релиз. Перед релизом переносим в `X-Y-Z.md` и очищаем этот файл.

Ниже — всё смысловое из коммитов с момента v0.8.0.

---

## EN

### Highlights
- **macOS TUN (privileged launch)** — RunWithPrivileges for TUN when running without full admin; correct sing-box PID and process-group stop; privileged start only when config has TUN (ConfigHasTun); reuse AuthorizationRef; privileged kill in "already running" dialog; fixes for config parse and wizard state.
- **Fix** — Config no longer corrupted after "Update" or timer refresh ([#31](https://github.com/Leadaxe/singbox-launcher/issues/31)): removed double comma in outbounds section (parser trailing comma + wizard comma after @ParserEND); single comma only when both dynamic and static blocks exist.
- **Refactoring** — `core/config/generator.go` → `outbound_generator.go`, `ui/wizard/business/generator.go` → `create_config.go`; simplified outbounds assembly (dynamic between @ParserSTART/@ParserEND + static, comma only when both present); all references and docs updated. **outbound_generator.go:** removed dead `GenerateSelector`; extracted three-pass logic into `buildOutboundsInfo`, `computeOutboundValidity`, `generateSelectorJSONs`; file-level and function-level documentation.
- **Template / get_free URL** — common `GetMyBranch()` for template and get_free.json; wizard uses develop branch when version has suffix, else main.
- **macOS installer** — auto-backup of wizard_states folder; installation script improvements.
- **Scripts** — download statistics tracking and script; latest release section, top 3 with medals.
- **Docs** — MACOS_TUN report updates (ConfigHasTun, wizard TUN section); telemetry concept (TELEMETRY_CONCEPT.md).
- **CI** — fix CI/CD.
- **Build (macOS)** — Version detection in `build_darwin.sh` aligned with Windows: support for `APP_VERSION` env var; `git describe --tags --always --dirty` for tag/branch; fallback `unnamed-dev` instead of hardcoded `0.4.1` when no tag available.

---

## RU

### Основное
- **TUN на macOS (привилегированный запуск)** — RunWithPrivileges для TUN без полных прав админа; корректный PID sing-box и остановка по группе процессов; привилегированный старт только при наличии TUN в конфиге (ConfigHasTun); переиспользование AuthorizationRef; привилегированный kill в диалоге «уже запущен»; исправления парсинга конфига и состояния визарда.
- **Исправление** — конфиг больше не портится после «Обновить» или обновления по таймеру ([#31](https://github.com/Leadaxe/singbox-launcher/issues/31)): убрана двойная запятая в секции outbounds (trailing comma парсера + запятая визарда после @ParserEND); одна запятая только при наличии обоих блоков (динамический и статический).
- **Рефакторинг** — переименование `core/config/generator.go` → `outbound_generator.go`, `ui/wizard/business/generator.go` → `create_config.go`; упрощённая сборка outbounds (динамические между маркерами и статические, запятая только при обоих блоках); обновлены упоминания и документация. **outbound_generator.go:** удалён неиспользуемый `GenerateSelector`; три прохода вынесены в `buildOutboundsInfo`, `computeOutboundValidity`, `generateSelectorJSONs`; добавлена документация в начале файла и к функциям.
- **Шаблон / URL get_free** — общая функция `GetMyBranch()` для шаблона и get_free.json; визард берёт ветку develop при суффиксе версии, иначе main.
- **Установщик macOS** — авто-бэкап папки wizard_states; улучшения скрипта установки.
- **Скрипты** — учёт статистики загрузок и скрипт; блок «последний релиз», топ-3 с медалями.
- **Документация** — обновления отчёта MACOS_TUN (ConfigHasTun, секция TUN в визарде); концепция телеметрии (TELEMETRY_CONCEPT.md).
- **CI** — исправление CI/CD.
- **Сборка (macOS)** — Определение версии в `build_darwin.sh` приведено к логике Windows: поддержка переменной окружения `APP_VERSION`; `git describe --tags --always --dirty` для тегов и веток; fallback `unnamed-dev` вместо жёсткого `0.4.1`, если тег недоступен.
