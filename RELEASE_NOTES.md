# Release Notes

## Последний релиз / Latest release

**v0.8.0** — полное описание (full details): [docs/release_notes/0-8-0.md](docs/release_notes/0-8-0.md)

## Что не вошло в релиз / Not yet released

Изменения после v0.8.0 (changes since v0.8.0): [upcoming.md](docs/release_notes/upcoming.md)

---

<details>
<summary><b>🇬🇧 English</b></summary>

### Highlights
- **macOS TUN** — privileged launch path for TUN on macOS; optional system helper for reliable TUN when running without full admin rights.
- **Refactoring** — `core/config/generator.go` → `outbound_generator.go`, `ui/wizard/business/generator.go` → `create_config.go`; simplified outbounds assembly (dynamic between @ParserSTART/@ParserEND + static, comma only when both present).

</details>

<details>
<summary><b>🇷🇺 Русская версия</b></summary>

### Основное
- **TUN на macOS** — привилегированный запуск для TUN на macOS; опциональный системный хелпер для стабильной работы TUN без полных прав администратора.
- **Рефакторинг** — переименование генераторов: `core/config/generator.go` → `outbound_generator.go`, `ui/wizard/business/generator.go` → `create_config.go`; упрощённая сборка outbounds (динамические между @ParserSTART/@ParserEND и статические, запятая только при наличии обоих блоков).

</details>
