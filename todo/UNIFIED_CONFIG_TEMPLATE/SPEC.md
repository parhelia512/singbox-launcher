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
7. **Обратная совместимость с state.json** — существующие `state.json` продолжают работать; связь `selectable_rule_states[i]` ↔ `selectable_rules[i]` — по `label`
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
6. Преобразовать `selectable_rules` в `[]TemplateSelectableRule`
7. Вернуть `TemplateData` (структура может быть обновлена, но внешний API сохранён)

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
| Формат `state.json` | Пользовательское состояние без изменений |

---

## Совместимость

### С state.json
- `state.json` хранит `selectable_rule_states` и `custom_rules` — это пользовательский выбор
- Связь: `state.json.selectable_rule_states[i]` ↔ `template.selectable_rules[i]` — по `label`
- При изменении порядка правил в шаблоне — маппинг по `label` корректно сопоставит

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

