# ТЗ: Единый config_template.json

## Проблема

### 1. Дублирование файлов шаблонов
Два файла — `bin/config_template.json` и `bin/config_template_macos.json` — содержат ~95% одинакового кода. Различаются только секцией `inbounds` и двумя TUN-правилами в `route.rules`. Любое изменение (добавление правила, outbound-группы, DNS-сервера) требует правки обоих файлов, что приводит к рассинхронизации.

### 2. Хрупкая система директив в комментариях
Текущий шаблон использует специальные комментарии как директивы:
- `/** @ParserConfig ... */` — конфигурация парсера
- `/** @SelectableRule @label ... @description ... @default ... */` — правила для визарда
- `/** @PARSER_OUTBOUNDS_BLOCK */` — маркер для вставки outbound-ов

Эти директивы парсятся регулярными выражениями (`extractCommentBlock`, `extractAllSelectableBlocks`, `parseSelectableRules`, `extractRuleMetadata`, `normalizeRuleJSON`). Проблемы:
- Регулярки хрупкие — ломаются от лишнего пробела, запятой, переноса строки
- Нет валидации структуры — ошибки обнаруживаются только в рантайме
- Правила и их зависимости (`rule_set`) разбросаны по разным местам файла
- Сложно добавлять новые метаданные к правилам

### 3. Отсутствие связи правило ↔ rule_set
В текущей структуре `rule_set` определения лежат в `route.rule_set`, а правила — в комментариях `@SelectableRule`. Нет явной связи между ними. Если пользователь отключил правило, его `rule_set` всё равно загружается и занимает трафик/ресурсы.

---

## Решение

Один файл `bin/config_template.json` с явной JSON-структурой из четырёх секций:

| Секция | Назначение |
|--------|-----------|
| `parser_config` | Конфигурация парсера (подписки, outbound-группы, интервал обновления) |
| `config` | Основной конфиг sing-box — платформонезависимая часть |
| `selectable_rules` | Правила маршрутизации для визарда (включение/выключение пользователем) |
| `params` | Платформозависимые параметры, применяемые к `config` |

---

## Критерии приёмки

1. **Один файл** — `bin/config_template_macos.json` удалён, `GetTemplateFileName()` возвращает одно имя
2. **Никаких директив в комментариях** — `@ParserConfig`, `@SelectableRule`, `@PARSER_OUTBOUNDS_BLOCK` не используются
3. **Никаких регулярок для парсинга** — `extractCommentBlock`, `extractAllSelectableBlocks`, `parseSelectableRules`, `extractRuleMetadata`, `normalizeRuleJSON` удалены
4. **Валидный JSON** — файл шаблона парсится стандартным `json.Unmarshal` без предварительной обработки
5. **Правило самодостаточно** — каждый `selectable_rule` содержит свои `rule_set` определения; при отключении правила его `rule_set` не попадает в конфиг
6. **Платформы через params** — `inbounds` и TUN-правила подставляются по `runtime.GOOS`, а не выбором файла
7. **Совместимость с state.json** — `selectable_rule_states` упрощается до `{ label, enabled, selected_outbound }`; миграция со старого формата через извлечение `label` из вложенного `rule.label`; связь с шаблоном — по `label`
8. **Существующая функциональность не нарушена** — визард загружает шаблон, показывает правила, генерирует конфиг, сохраняет, обновляет

---

## Требования к секциям

### `parser_config`
- Формат без изменений от текущего `@ParserConfig`
- Содержит `version`, `proxies`, `outbounds`, `parser`
- Outbound-группы с полем `wizard` (`required`, `hide`) — логика без изменений
- Парсится напрямую из JSON, не из комментариев

### `config`
- Полный конфиг sing-box: `log`, `dns`, `inbounds`, `outbounds`, `route`, `experimental`
- `inbounds` — пустой массив `[]`, заполняется из `params` по платформе
- `outbounds` — только статический `direct-out`; сгенерированные парсером outbound-группы вставляются в начало массива (логика `@PARSER_OUTBOUNDS_BLOCK` заменяется: outbounds всегда вставляются перед `direct-out`)
- `route.rules` — только базовые универсальные правила (hijack-dns, ip_is_private, local); selectable и TUN правила — отдельно
- `route.rule_set` — только общие rule_set, используемые несколькими правилами или DNS-правилами (например `ru-domains`); остальные привязаны к `selectable_rules`

### `selectable_rules`
- Массив объектов, каждый описывает одно правило для визарда
- Обязательные поля: `label` (название в UI), `description` (tooltip)
- Опциональные поля:
  - `default` (bool) — включено по умолчанию при первом запуске
  - `platforms` (array of string) — платформы, на которых правило доступно (`"windows"`, `"linux"`, `"darwin"`). Если не указано — правило доступно на всех платформах. Если указано — правило отображается в визарде и применяется только на перечисленных платформах
  - `rule_set` (array) — определения rule_set, необходимые для работы правила; добавляются в `config.route.rule_set` только если правило включено
  - `rule` (object) — одиночное правило маршрутизации
  - `rules` (array) — несколько правил (взаимоисключающее с `rule`)
- Правила без собственных rule_set (по `domain`, `port`, `protocol`) просто не имеют поля `rule_set`
- Правила, ссылающиеся на общий rule_set (например `ru-domains` из `config.route.rule_set`), не дублируют его определение

### `params`
- Массив платформозависимых параметров
- Обязательные поля: `name` (путь в config, точечная нотация), `platforms` (массив платформ), `value` (значение)
- Опциональное поле: `mode` — режим применения:
  - `"replace"` (по умолчанию) — заменить значение `config[name]` на `value`
  - `"prepend"` — вставить элементы `value` в начало массива `config[name]`
  - `"append"` — добавить элементы `value` в конец массива `config[name]`
- Маппинг платформ: `"windows"` → `windows`, `"linux"` → `linux`, `"darwin"` → `darwin`
- Применяется при загрузке шаблона: определяется `runtime.GOOS`, для каждого param проверяется `platforms`, если совпадает — применяется `value` к `config`

---

## Требования к загрузке шаблона

### Алгоритм LoadTemplateData (новый)
Всегда читается один файл `bin/config_template.json` — без зависимости от платформы.
Платформенные различия применяются на этапе обработки `params`.

1. Прочитать `bin/config_template.json` (один файл для всех ОС)
2. `json.Unmarshal` в структуру `UnifiedTemplate` (4 поля: `parser_config`, `config`, `selectable_rules`, `params`)
3. Определить `runtime.GOOS`
4. Применить `params`: для каждого param, если текущая ОС в `platforms`, применить `value` к `config` по пути `name` с учётом `mode`
5. Извлечь `DefaultFinal` из `config.route.final`
6. Отфильтровать `selectable_rules` по `platforms` (`runtime.GOOS`) — правила, не предназначенные для текущей платформы, отбрасываются **до** маппинга с `state.json`
7. Преобразовать отфильтрованные `selectable_rules` в `[]TemplateSelectableRule`
8. Вернуть `TemplateData` (структура может быть обновлена, но внешний API сохранён)

---

## Генерация финального конфига

`config` из шаблона читается как есть и обогащается из `params` с учетом по платформы которая в `runtime.GOOS`.
`selectable_rules` отображаются в визарде — пользователь включает/выключает.

При сохранении конфига:
- Включённые (`enabled`) selectable_rules добавляются в `config.route.rules`
- Их `rule_set` (если есть) добавляются в `config.route.rule_set`
- Custom rules пользователя добавляются после selectable
- Outbound-группы из парсера вставляются перед статическими `config.outbounds`
- Дедупликация rule_set по `tag`

---

## Нормализация process_name
- Благодаря полю `platforms` в `selectable_rules`, каждая платформа хранит свои имена процессов явно в шаблоне
- Windows: `Telegram.exe`, `Zoom.exe`; macOS: `Telegram`, `zoom.us`; Linux: `telegram-desktop` и т.д.
- Нормализация в коде больше не нужна — шаблон самодостаточен

---

## Что удаляется

| Элемент | Причина |
|---------|---------|
| `bin/config_template_macos.json` | Заменён единым файлом |
| `GetTemplateFileName()` if/else по GOOS | Возвращает одно имя |
| `GetTemplateURL()` if/else по GOOS | Один URL |
| `extractCommentBlock()` | Не нужна — JSON парсится напрямую |
| `extractAllSelectableBlocks()` | Не нужна — правила в отдельной секции |
| `parseSelectableRules()` | Не нужна — правила парсятся как обычный JSON |
| `extractRuleMetadata()` | Не нужна — метаданные в полях JSON |
| `normalizeRuleJSON()` | Не нужна — JSON уже валидный |
| `extractOutboundsAfterMarker()` | Не нужна — outbound-ы в `config.outbounds` |
| Директивы `@ParserConfig`, `@SelectableRule`, `@PARSER_OUTBOUNDS_BLOCK` | Заменены структурой JSON |
| `cloneRule()` + `normalizeProcessNames()` | Не нужны — `platforms` в `selectable_rules` хранит правильные имена процессов для каждой платформы |

---

## Что остаётся без изменений

| Элемент | Причина |
|---------|---------|
| `MergeRouteSection()` | Объединение базовых + selectable + custom rules |
| `BuildParserOutboundsBlock()` | Генерация outbound-блока из подписок |
| `EnsureRequiredOutbounds()` | Проверка обязательных outbound-ов из шаблона |

---

## Изменения в state.json

### Проблема: дублирование данных шаблона в state.json

Текущая структура `selectable_rule_states` в `state.json` дублирует данные из шаблона:
```json
{
  "type": "System",
  "rule": {
    "label": "Block ads",
    "description": "Block advertisement domains",
    "raw": { "domain_suffix": ["ads.example.com"], "action": "reject" },
    "default_outbound": "reject",
    "has_outbound": false,
    "is_default": true
  },
  "enabled": true,
  "selected_outbound": "reject"
}
```

Поля `description`, `raw`, `default_outbound`, `has_outbound`, `is_default` — всё это уже есть в шаблоне (`selectable_rules`). Хранить копию в state → рассинхронизация при обновлении шаблона.

### Решение: state хранит только выбор пользователя

Новая структура `selectable_rule_states`:
```json
{
  "label": "Block ads",
  "enabled": true,
  "selected_outbound": "reject"
}
```

Три поля вместо вложенной структуры:
- `label` (string) — ключ для маппинга с `selectable_rules[i].label` из шаблона
- `enabled` (boolean) — включено ли правило
- `selected_outbound` (string) — выбранный outbound

Всё остальное (`description`, `rule`/`rules`, `rule_set`, `default`, `platforms`) берётся из шаблона при загрузке.

### Что это даёт
- **Нет дублирования** — шаблон единственный источник правды для определений правил
- **Нет рассинхронизации** — при обновлении шаблона правила автоматически обновляются
- **Меньше размер** state.json
- **Устраняется путаница** `rule` vs `rule.raw` — в шаблоне `rule` это само правило маршрутизации, в старом state `rule` был обёрткой с метаданными

### platforms — прозрачно для state
State не хранит информацию о платформах. При загрузке шаблона фильтрация `selectable_rules` по `platforms` (`runtime.GOOS`) происходит **до** маппинга с `state.json`. Сначала отбрасываются правила, не предназначенные для текущей платформы (например, на `"windows"` — правила с `"platforms": ["darwin", "linux"]`), затем оставшиеся правила сопоставляются с `state.json` по `label`. В визарде показываются только правила для текущей платформы. State хранит `label` + выбор пользователя.

### rule_set — не хранится в state
Определения `rule_set` привязаны к `selectable_rules` в шаблоне. Включил правило → его `rule_set` добавляется в финальный конфиг. State не дублирует определения rule_set.

### custom_rules — без изменений
Пользовательские правила (`custom_rules`) по-прежнему хранят полную структуру, т.к. они не привязаны к шаблону. Структура `custom_rules` остаётся прежней.

### Миграция
При загрузке state.json старого формата:
- Если элемент `selectable_rule_states[i]` содержит вложенный `rule.label` — извлечь `label` из `rule.label`
- Если элемент уже содержит `label` на верхнем уровне — использовать как есть
- Маппинг `label` → `selectable_rules[i]` по совпадению

### Обновление WIZARD_STATE_JSON_SCHEMA.md
Документ `todo/complete/WIZARD_STATE/WIZARD_STATE_JSON_SCHEMA.md` требует обновления:

| Что | Было | Стало |
|-----|------|-------|
| `selectable_rule_states` элемент | Вложенная структура `PersistedRuleState` с `type`, `rule { label, description, raw, ... }`, `enabled`, `selected_outbound` | Плоская структура `{ label, enabled, selected_outbound }` |
| Поле `type` в selectable | `"System"` / `"IP Addresses (CIDR)"` / ... | Убрано — все selectable правила берут определение из шаблона |
| Поле `rule.raw` | Полный JSON правила | Убрано — правило в шаблоне (`selectable_rules[i].rule`) |
| Поле `rule.description` | Описание правила | Убрано — описание в шаблоне (`selectable_rules[i].description`) |
| Поля `default_outbound`, `has_outbound`, `is_default` | Метаданные из шаблона | Убраны — берутся из шаблона |
| Ссылки на `@SelectableRule` блоки | Множественные | Убраны — правила в секции `selectable_rules` |
| Ссылки на два файла шаблонов | `config_template.json` **или** `config_template_macos.json` | Один файл `bin/config_template.json` |
| Ссылки на `@ParserConfig`, `@PARSER_OUTBOUNDS_BLOCK` | Множественные | Убраны — парсится как JSON |
| Секция "Что НЕ сохраняется" | `TemplateData` с `Sections`, `SectionOrder`, `HasParserOutboundsBlock`, `OutboundsAfterMarker` | `TemplateData` с `Config`, `SelectableRules`, `Params` |
| Восстановление состояния | Загрузка `rule.raw` из state | Загрузка только `label`/`enabled`/`selected_outbound`, определения правил из шаблона |

---

## Совместимость

### С state.json
- `state.json` хранит `selectable_rule_states` — только выбор пользователя (`label`, `enabled`, `selected_outbound`)
- Связь: `state.json.selectable_rule_states[i].label` ↔ `template.selectable_rules[i].label`
- При изменении порядка правил в шаблоне — маппинг по `label` корректно сопоставит
- При миграции со старого формата: `label` извлекается из вложенного `rule.label`
- `custom_rules` — структура не меняется

### С существующим config.json
- `parser.ExtractParserConfig()` продолжает работать с `config.json` для чтения ParserConfig из существующих конфигов (обратная совместимость)
- Новые конфиги генерируются уже из нового шаблона

### С системой обновления шаблонов
- `GetTemplateURL()` возвращает один URL
- Автообновление скачивает один файл вместо двух

---

## Особенности реализации, которые надо учесть

### ParserConfig больше не строка
В старой реализации `ParserConfig` извлекался из комментария регуляркой и хранился как сырая строка (`string`). В новой — `parser_config` это полноценный JSON-объект в структуре шаблона. Парсится напрямую через `json.Unmarshal` в Go-структуру. Промежуточная сериализация в строку не нужна.

**Влияние:** все места, которые работают с `TemplateData.ParserConfig` как со строкой (`string`), должны быть переведены на работу с типизированной структурой.

### TemplateData — обновлённая структура
Текущая `TemplateData` содержит поля, завязанные на старый формат:
- `ParserConfig string` → заменяется на типизированный объект
- `Sections map[string]json.RawMessage` + `SectionOrder []string` → заменяется на `Config` (единый JSON-объект из секции `config`)
- `HasParserOutboundsBlock bool` → не нужен: outbound-ы всегда вставляются перед статическими элементами `config.outbounds`
- `OutboundsAfterMarker string` → не нужен: статические outbound-ы берутся из `config.outbounds`

Добавляются:
- Массив `SelectableRules` с полем `RuleSets` — определения rule_set, привязанные к правилу
- `Config` — JSON-объект конфига после применения `params`

### process_name — нормализация больше не нужна
Благодаря `platforms` в `selectable_rules` каждая платформа хранит свои имена процессов явно. `cloneRule()` и `normalizeProcessNames()` удаляются — шаблон самодостаточен.

### Порядок применения params
Params применяются в порядке их следования в массиве. Если для одной секции и платформы есть несколько params, они применяются последовательно. Для `mode: "replace"` последний перезапишет предыдущий. Для `mode: "prepend"/"append"` — элементы накапливаются.

### Дедупликация rule_set
При включении нескольких `selectable_rules`, ссылающихся на один и тот же `rule_set` (например, несколько правил используют `ru-domains` из `config.route.rule_set`), дедупликация по `tag` обязательна — один `rule_set` не должен дублироваться в финальном конфиге.

### state.json — упрощение selectable_rule_states
Структура `selectable_rule_states` в `state.json` упрощается до `{ label, enabled, selected_outbound }`.
Шаблон — единственный источник правды для определений правил. State хранит только выбор пользователя.
Подробности — в разделе «Изменения в state.json».

---

## Обновление документации

Переход на единый шаблон затрагивает множество документов. Ниже — полный список файлов и конкретные изменения в каждом.

### 1. `todo/complete/WIZARD_STATE/WIZARD_STATE_JSON_SCHEMA.md`

**Приоритет:** высокий (формат данных)

Что обновить:
- **`selectable_rule_states`** — заменить `PersistedRuleState` (вложенная структура с `type`, `rule { label, description, raw, ... }`) на плоскую `{ label, enabled, selected_outbound }`
- **Примеры JSON** — обновить все примеры state.json с новым форматом
- **Секция "Восстановление состояния"** — описать новый алгоритм:
  - Загрузка шаблона (один файл `bin/config_template.json`)
  - Маппинг `selectable_rule_states[i].label` → `selectable_rules[i].label` из шаблона
  - Определения правил берутся из шаблона, не из state
- **Ссылки на `@SelectableRule`, `@ParserConfig`, `@PARSER_OUTBOUNDS_BLOCK`** — убрать, заменить на описание JSON-секций
- **Ссылки на два файла шаблонов** (`config_template.json` или `config_template_macos.json`) — заменить на один `bin/config_template.json`
- **Секция "Что НЕ сохраняется"** — обновить `TemplateData`:
  - Убрать: `Sections`, `SectionOrder`, `HasParserOutboundsBlock`, `OutboundsAfterMarker`
  - Добавить: `Config`, `SelectableRules` (с `RuleSets`), `Params`
- **Секция "Режим «Настроить заново»"** — обновить: правила инициализируются из `selectable_rules` (JSON), а не из `@SelectableRule` блоков
- **Миграция** — добавить описание миграции со старого формата state.json

### 2. `docs/ARCHITECTURE.md`

**Приоритет:** высокий (архитектурная документация)

Что обновить:
- **`ui/wizard/template/loader.go`** описание:
  - Убрать: «Извлечение @ParserConfig, @SelectableRule блоков», «Извлечение специальных блоков: @ParserConfig, @SelectableRule, @PARSER_OUTBOUNDS_BLOCK»
  - Заменить на: «Загрузка единого шаблона, применение params по платформе, парсинг selectable_rules»
  - `TemplateData struct` — обновить описание полей
  - `GetTemplateFileName()` — убрать упоминание платформозависимого выбора
  - `GetTemplateURL()` — один URL
- **`ui/wizard/business/saver.go`** описание:
  - Убрать: `NextBackupPath()` (если уже удалён)
  - Обновить `FileServiceAdapter` описание
- **`ui/wizard/business/loader.go`** описание:
  - Обновить `LoadConfigFromFile()` — учесть новый формат шаблона
- **`ui/wizard/business/generator.go`** описание:
  - `BuildTemplateConfig()` — работает с `Config` объектом, а не `Sections`
  - `MergeRouteSection()` — добавляет rule_set из selectable_rules
- **`ui/wizard/models/wizard_state_file.go`** описание:
  - `PersistedRuleState struct` — обновить до плоской структуры
- **Секция "Зоны ответственности"** → **template/**:
  - Убрать: «Извлечение @ParserConfig, @SelectableRule блоков»
  - Добавить: «Загрузка единого шаблона с 4 секциями, применение params»
- **Удалённые функции** из таблицы "Что удаляется" (SPEC) — убрать из дерева файлов, если присутствуют:
  - `extractCommentBlock`, `extractAllSelectableBlocks`, `parseSelectableRules`, `extractRuleMetadata`, `normalizeRuleJSON`, `extractOutboundsAfterMarker`
  - `cloneRule()`, `normalizeProcessNames()`
  - `GetRequiredFiles()`

### 3. `docs/CREATE_WIZARD_TEMPLATE.md` (English)

**Приоритет:** высокий (руководство для провайдеров)

**Полная переработка** — документ описывает старую систему директив в комментариях.

Что обновить:
- **"Understanding the Special Markers"** — полностью заменить. Убрать секции `@ParcerConfig`, `@PARSER_OUTBOUNDS_BLOCK`, `@SelectableRule`. Описать 4 JSON-секции: `parser_config`, `config`, `selectable_rules`, `params`
- **"Quick Start" / "Step 2: Use This Minimal Template"** — заменить пример JSONC-шаблона с комментариями на новый JSON-шаблон с 4 секциями
- **"Example Rules"** — обновить примеры: правила теперь в секции `selectable_rules` с полями `label`, `description`, `default`, `platforms`, `rule_set`, `rule`/`rules`
- **"Complete Example: Real-World Template"** — полностью переписать на новый формат (можно взять за основу `todo/UNIFIED_CONFIG_TEMPLATE/config_template.json`)
- **"Testing Your Template"** — обновить проверки: JSON парсится стандартно, без JSONC
- **"Troubleshooting"** — обновить типичные проблемы: убрать «@SelectableRule not found», добавить проблемы с `params` и `platforms`
- **"Best Practices"** — обновить рекомендации: один файл, правила самодостаточны, rule_set при правиле
- **Платформы** — добавить раздел про `params` (платформозависимые inbounds, route.rules) и `platforms` в `selectable_rules`
- **TUN vs System Proxy** — обновить: inbounds задаются через `params`, а не статически в шаблоне

### 4. `docs/CREATE_WIZARD_TEMPLATE_RU.md` (Русский)

**Приоритет:** высокий (русская версия руководства)

Аналогичная полная переработка, как для `docs/CREATE_WIZARD_TEMPLATE.md`. Все те же изменения, но на русском языке. Документ — перевод английской версии, поэтому структура изменений идентична.

### 5. `README.md` (English)

**Приоритет:** средний

Что обновить:
- **"Config Template (config_template.json)"** — обновить описание: один файл вместо двух, JSON без комментариев-директив, 4 секции
- **"Subscription Parser Configuration"** — обновить: `parser_config` теперь секция в шаблоне, а не комментарий `@ParserConfig`
- Если есть упоминания `config_template_macos.json` — убрать
- Если есть примеры с `@SelectableRule`, `@PARSER_OUTBOUNDS_BLOCK` — заменить на новый формат

### 6. `README_RU.md` (Русский)

**Приоритет:** средний

Аналогичные изменения, как для `README.md`, на русском языке. Все упоминания директив в комментариях, двух файлов шаблонов и старого формата обновляются.

### Порядок обновления документации

1. Сначала — `WIZARD_STATE_JSON_SCHEMA.md` (определяет формат данных)
2. Затем — `CREATE_WIZARD_TEMPLATE.md` + `CREATE_WIZARD_TEMPLATE_RU.md` (основные руководства)
3. Затем — `ARCHITECTURE.md` (архитектурная документация)
4. В конце — `README.md` + `README_RU.md` (обзорная документация)

---

## Оценка задачи

### Важность: высокая

Задача затрагивает **фундамент** работы визарда — формат шаблона определяет, как создаются конфиги, как провайдеры описывают свои сервисы, как пользователь взаимодействует с правилами. Текущая реализация на комментариях-директивах и регулярках — техдолг, который усложняет каждое изменение и делает систему хрупкой. Чем дольше откладывать, тем больше кода завязывается на старый формат и тем дороже миграция.

### Что решает сейчас

| Проблема | Масштаб | Как решается |
|----------|---------|-------------|
| Два файла шаблонов с 95% дублированием | Каждое изменение × 2, рассинхронизация | Один файл + `params` |
| Регулярки для парсинга комментариев | Хрупкость, скрытые баги, сложная отладка | Стандартный `json.Unmarshal` |
| rule_set грузятся даже для отключённых правил | Лишний трафик и ресурсы | rule_set привязан к правилу |
| Дублирование данных шаблона в state.json | Рассинхронизация при обновлении шаблона | State хранит только выбор пользователя |
| Нормализация process_name в коде | Неочевидная логика, скрытые зависимости | Платформенные имена явно в шаблоне |

### Что открывает на будущее

- **Новые метаданные правил** — добавить поле в `selectable_rules` = одна строка в JSON, без правки регулярок и парсеров
- **Новые платформы** (Android, iOS) — добавить платформу в `params` и `platforms`, без нового файла шаблона и без нового кода
- **Валидация шаблона** — JSON Schema позволяет проверить шаблон до загрузки в приложение, в CI, в редакторе провайдера
- **Внешние провайдеры** — чистый JSON с 4 секциями проще документировать и проще создавать, чем формат с директивами в комментариях
- **Версионирование шаблона** — можно добавить `"version"` на верхний уровень и обрабатывать миграцию между версиями формата
- **Композитные шаблоны** — в перспективе можно собирать шаблон из нескольких источников (базовый + расширение провайдера), т.к. структура предсказуема

### Риски

| Риск | Вероятность | Митигация |
|------|-------------|-----------|
| Поломка визарда при переходе | Средняя | Миграция state.json, ручное тестирование |
| Несовместимость со старыми шаблонами от провайдеров | Низкая (провайдеров пока мало) | Документация + период уведомления |
| Регрессия в генерации конфига | Средняя | Тесты на эталонный конфиг (golden tests) |
| Объём изменений (~15 файлов) | — | Порядок реализации из SPEC, атомарные шаги |

### Вывод

Задача не «было бы неплохо» — это **необходимость**. Текущий формат достиг предела: добавление нового правила или поддержка новой платформы требует изменений в нескольких местах с риском сломать парсинг. Новый формат устраняет целый класс багов (регулярки, рассинхронизация файлов, дублирование данных) и превращает шаблон из «внутреннего файла с хаками» в **документированный API** с JSON Schema. Объём работы значительный, но каждый удалённый парсер комментариев — это код, который больше не надо поддерживать.

