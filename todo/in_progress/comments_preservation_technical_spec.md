# Техническое задание: Сохранение комментариев при генерации конфига из Wizard

## Ссылки на связанные документы

- **Анализ проблемы:** `todo/in_progress/comments_preservation_analysis.md`
- **Исходная задача:** Сохранение всех комментариев из шаблона при создании конфига через Wizard

## Обзор проблемы

При создании конфигурации через Wizard теряются все комментарии из исходного шаблона. Проблема возникает на нескольких этапах обработки:

1. **Форматирование секций** - `FormatSectionJSON` использует `json.Indent`, который удаляет комментарии
2. **Слияние route секции** - `MergeRouteSection` использует `json.Unmarshal`/`json.Marshal`, теряя комментарии
3. **Сохранение конфига** - `SaveConfigWithBackup` использует `json.MarshalIndent` при замене secret

**Примечание:** Следующие части конфига **НЕ требуют** специальной обработки:
- **@ParserConfig блок** - заменяется полностью, проблем нет
- **Поле "comment" в outbounds** - это JSON поле, не комментарий, сохраняется автоматически
- **@PARSER_OUTBOUNDS_BLOCK** - обрабатывается текстово, проблем нет

Комментарии составляют 40-50% содержимого конфигов и критически важны для понимания конфигурации.

## Выбранный подход

**Текстовый подход:** Работа с исходным текстом напрямую, без полного парсинга JSON.

**Основные принципы:**
- Сохраняем исходный JSONC текст секций с комментариями
- Меняем только то, что нужно изменить (/**...*/ блоки)
- Для @SelectableRule блоков делаем текстовые замены (включено/выключено)
- Для других секций - возвращаем текст как есть (без форматирования)
- Используем JSON парсинг только для валидации и извлечения метаданных
- Избегаем `json.Unmarshal`/`json.Marshal` для финального конфига

**Преимущества:**
- Максимальное сохранение комментариев (не парсим/не сериализуем JSON)
- Простая реализация (текстовые замены)
- Меньше точек потери комментариев
- Быстрая разработка

## Используемые библиотеки

### `github.com/muhammadmuzzammil1998/jsonc`

**Использование:**
- `jsonc.ToJSON` - конвертация JSONC в чистый JSON (для валидации)
- `jsonc.Valid` - проверка валидности JSONC

**Ограничение:** Библиотека не сохраняет комментарии при форматировании, поэтому используется только для валидации.

### Поиск библиотек для сохранения комментариев

**Результат поиска:** Готовых библиотек для Go, которые позволяют извлекать секции из JSONC с сохранением комментариев (что-то вроде `config.dns.getText()`), **не найдено**.

**Существующие библиотеки:**
- `github.com/muhammadmuzzammil1998/jsonc` - только конвертация JSONC → JSON (теряет комментарии)
- Другие библиотеки JSONC для Go также фокусируются на валидации и конвертации, а не на сохранении комментариев

**Вывод:** Необходимо реализовать текстовое извлечение секций самостоятельно, используя парсинг JSONC с подсчетом скобок (как описано в разделе "Как извлечь секцию текстово").

## Решение

#### Архитектура решения

```
┌─────────────────────────────────────────────────────────┐
│  Исходный шаблон (JSONC с комментариями)                │
└──────────────────┬──────────────────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────────────────┐
│  Загрузка шаблона                                        │
│  - Чтение исходного текста (JSONC)                       │
│  - Извлечение метаданных из @SelectableRule блоков       │
│  - Сохранение исходного текста секций с блоками         │
│  - Блоки остаются в тексте для последующей замены        │
└──────────────────┬───────────────────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────────────────┐
│  Хранение                                                │
│  - TemplateData.Sections[key] = исходный JSONC текст     │
│  - TemplateSelectableRules = метаданные для UI           │
└──────────────────┬───────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────┐
│  Генерация конфига                                      │
│  - Для route секции: замена @SelectableRule блоков      │
│    на обработанные версии (включено/выключено)          │
│  - Для других секций: возврат текста как есть          │
│  - Сохранение всех комментариев                         │
└──────────────────┬──────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────┐
│  Финальный конфиг (JSONC с комментариями)               │
└─────────────────────────────────────────────────────────┘
```

#### Компоненты решения

**1. TextTemplateSection - хранение исходного текста секции**

```go
// TemplateData уже хранит Sections как json.RawMessage (исходный JSONC текст)
// Это идеально подходит для текстового подхода
type TemplateData struct {
    Sections map[string]json.RawMessage // Исходный JSONC текст секции целиком
    // Ключ - имя секции (log, dns, inbounds, route, outbounds, experimental)
    // Значение - весь текст секции как есть, со всеми комментариями и вложенными структурами
    // ... остальные поля
}
```

**Важно:** Каждая секция хранится целиком как один текстовый блок. Сложные вложенные структуры (например, массив объектов в `inbounds`) не разбиваются на части - вся секция сохраняется и возвращается как есть.

**2. SelectableRuleTextProcessor - обработка @SelectableRule блоков**

```go
type SelectableRuleTextProcessor struct{}

// Обрабатывает @SelectableRule блок в тексте
func (p *SelectableRuleTextProcessor) ProcessRuleBlock(
    templateText string,
    ruleBlock string,
    ruleState *RuleState,
) (string, error)

// Удаляет дублирующий JSON после блока (если есть)
func (p *SelectableRuleTextProcessor) RemoveDuplicateJSON(
    text string,
    ruleBlock string,
) string
```

**3. RouteSectionTextMerger - слияние route секции через текст**

```go
type RouteSectionTextMerger struct{}

// Объединяет правила маршрутизации через текстовые операции
func (m *RouteSectionTextMerger) MergeRouteSection(
    templateText string,        // Исходный JSONC текст route секции
    ruleStates []*RuleState,    // Состояния правил из UI
    customRules []*RuleState,   // Пользовательские правила
    finalOutbound string,       // Финальный outbound
) (string, error)
```


#### Алгоритм работы

**Этап 1: Загрузка шаблона**

1. Читаем исходный файл как текст (JSONC)
2. Извлекаем метаданные из @SelectableRule блоков для UI:
   - Парсим блоки для получения label, description, default
   - Сохраняем в `TemplateSelectableRules`
3. **Блоки остаются в тексте** (в отличие от текущей реализации, где они удаляются)
4. Для каждой секции:
   - **Извлекаем секцию текстово** из исходного JSONC файла (сохраняя комментарии)
   - Сохраняем исходный JSONC текст секции в `TemplateData.Sections[key]`
   - Валидируем JSON (через `jsonc.ToJSON` + `json.Valid`) только для проверки
   - **Сохраняем исходный текст с комментариями**

**Как извлечь секцию текстово (например, `dns` с вложенной структурой и комментариями)?**

Используем текстовый парсинг для извлечения секции из JSONC:

**Алгоритм:**
1. Находим начало секции по паттерну `"dns": {` или `"dns": [` (с учетом комментариев перед ключом)
2. Парсим вложенные структуры, считая скобки `{`/`}` или `[`/`]`
3. Игнорируем комментарии при подсчете скобок (но сохраняем их в результате)
4. Находим закрывающую скобку соответствующего уровня
5. Извлекаем весь текст от начала до конца секции

**Детали реализации:** См. код в `ui/wizard/template/loader.go` - функция `extractSectionText`.

**Результат:** Весь текст секции от `"dns": {` до соответствующей `}`, включая все комментарии и вложенные структуры.

**Важно:** 
- В текущей реализации используется `parseJSONWithOrder`, который использует `json.Unmarshal` - это теряет комментарии
- Готовых библиотек для Go, которые сохраняют комментарии и позволяют извлекать секции как текст (типа `config.dns.getText()`), **не существует** (см. раздел "Поиск библиотек для сохранения комментариев")
- Необходимо реализовать текстовое извлечение секций самостоятельно, используя парсинг JSONC с подсчетом скобок, как показано выше
- Это стандартный подход для работы с JSONC, когда нужно сохранить комментарии


**Зачем поблочное хранение (Sections map[string]json.RawMessage)?**

Поблочное хранение секций необходимо для:

1. **Независимой обработки секций:**
   - Каждая секция обрабатывается по-своему: `route` - слияние правил, `outbounds` - генерация, остальные - как есть
   - Можно применять разную логику к разным секциям без усложнения кода

2. **Выборочного включения секций:**
   - Пользователь может выбрать, какие секции включить в финальный конфиг
   - Необработанные секции просто пропускаются

3. **Сохранения порядка:**
   - `SectionOrder` определяет порядок секций в финальном конфиге
   - Можно переупорядочивать секции независимо от их порядка в шаблоне

4. **Изоляции изменений:**
   - Изменения в одной секции (например, route) не влияют на другие
   - Ошибка в одной секции не ломает обработку остальных

5. **Удобства работы с текстом:**
   - Каждая секция - отдельный JSONC текст, можно работать с ним напрямую
   - Не нужно парсить весь конфиг целиком для изменения одной секции

**Этап 2: Обработка @SelectableRule блоков в route секции**

**Логика:** Заменяем каждый @SelectableRule блок на обработанную версию в зависимости от состояния правила.

```go
func ProcessSelectableRuleBlocks(
    templateText string,        // Исходный JSONC текст route секции с блоками
    ruleStates []*RuleState,    // Состояния правил из UI
) (string, error) {
    // Для каждого @SelectableRule блока:
    // 1. Находим блок в тексте
    // 2. Извлекаем JSON правило из блока
    // 3. Проверяем состояние правила (enabled/disabled)
    // 4. Заменяем блок на обработанную версию:
    //    - Если правило выключено: блок с @default, без JSON после блока
    //    - Если правило включено: блок + JSON правило после блока
    // 5. Удаляем дублирующий JSON после блока (если есть)
}
```

**Пример обработки:**

**Пример 1: Правило с `action` (блокировка)**

**Исходный шаблон:**
```json
/**   @SelectableRule
      @label Block Ads
      @description Soft-block ads
      {"rule_set": "ads-all", "action": "reject"},
*/
    {"rule_set": "ads-all", "action": "reject"},
```

**Если правило выключено:**
```json
/**   @SelectableRule
      @label Block Ads
      @default
      @description Soft-block ads
      {"rule_set": "ads-all", "action": "reject"},
*/
```
(JSON после блока удален, @default добавлен в блок)

**Если правило включено:**
```json
/**   @SelectableRule
      @label Block Ads
      @description Soft-block ads
      {"rule_set": "ads-all", "action": "reject"},
*/
    {"rule_set": "ads-all", "action": "reject"},
```
(JSON после блока остается или заменяется на новое правило)

**Пример 2: Правило с `outbound` (маршрутизация)**

**Исходный шаблон:**
```json
/**   @SelectableRule
      @label Russian domains direct
      @description Route Russian domains
      { "rule_set": "ru-domains", "outbound": "direct-out" },
*/
    { "rule_set": "ru-domains", "outbound": "direct-out" },
```

**Если правило выключено:**
```json
/**   @SelectableRule
      @label Russian domains direct
      @default
      @description Route Russian domains
      { "rule_set": "ru-domains", "outbound": "direct-out" },
*/
```
(JSON после блока удален, @default добавлен в блок)

**Если правило включено:**
```json
/**   @SelectableRule
      @label Russian domains direct
      @description Route Russian domains
      { "rule_set": "ru-domains", "outbound": "direct-out" },
*/
    { "rule_set": "ru-domains", "outbound": "proxy-out" },
```
(JSON после блока заменен на новое правило с учетом выбранного outbound из UI)

**Этап 3: Обработка секций (log, dns, inbounds, experimental)**

**Принцип:** Сложные вложенные структуры (например, `inbounds` - массив объектов с множеством полей и комментариев) хранятся и возвращаются целиком как один текстовый блок, без изменений. Мы не шаблонизируем внутренние структуры - только работаем с целыми секциями.

```go
func ProcessSection(
    raw json.RawMessage, // Исходный JSONC текст секции (целиком)
) (string, error) {
    text := string(raw)
    
    // 1. Валидируем JSONC (только для проверки)
    if !jsonc.Valid([]byte(text)) {
        return "", fmt.Errorf("invalid JSONC")
    }
    
    // 2. Возвращаем текст как есть - сохраняем все комментарии и форматирование
    // Для inbounds это может быть сложная структура типа:
    // "inbounds": [
    //   // Комментарий
    //   {
    //     "type": "tun",
    //     "tag": "tun-in",
    //     // ... множество полей с комментариями
    //   }
    // ]
    // Вся эта структура возвращается как есть, без изменений
    return text, nil
}
```


**Этап 4: Слияние route секции (текстовый подход)**

```go
func MergeRouteSectionText(
    templateText string,        // Исходный JSONC текст route секции
    ruleStates []*RuleState,    // Состояния правил из UI
    customRules []*RuleState,   // Пользовательские правила
    finalOutbound string,       // Финальный outbound
) (string, error) {
    // 1. Обрабатываем @SelectableRule блоки (текстовые замены)
    result := ProcessSelectableRuleBlocks(templateText, ruleStates)
    
    // 2. Добавляем пользовательские правила (текстово, после существующих)
    if len(customRules) > 0 {
        result = AddCustomRules(result, customRules)
    }
    
    // 3. Обновляем final outbound (текстовая замена)
    if finalOutbound != "" {
        result = UpdateFinalOutbound(result, finalOutbound)
    }
    
    // 4. Валидируем результат
    if !jsonc.Valid([]byte(result)) {
        return "", fmt.Errorf("invalid JSONC after merge")
    }
    
    return result, nil
}
```

**Этап 5: Сохранение конфига**

```go
func SaveConfigWithComments(
    configText string, // JSONC текст с комментариями
    secret string,
) (string, error) {
    finalText := configText
    
    // 1. Заменяем константу CHANGE_THIS_TO_YOUR_SECRET_TOKEN на случайный secret (если есть)
    //    Это необязательная операция - просто текстовая замена константы
    changeTokenPattern := regexp.MustCompile(`("secret"\s*:\s*)"CHANGE_THIS_TO_YOUR_SECRET_TOKEN"`)
    if changeTokenPattern.MatchString(finalText) {
        finalText = changeTokenPattern.ReplaceAllString(
            finalText, 
            fmt.Sprintf(`$1"%s"`, secret),
        )
    }
    
    // 2. Заменяем существующий secret на новый (если есть)
    //    Ищем "secret" в experimental.clash_api секции
    secretPattern := regexp.MustCompile(`("secret"\s*:\s*)"[^"]*"`)
    if secretPattern.MatchString(finalText) && strings.Contains(finalText, "clash_api") {
        finalText = secretPattern.ReplaceAllString(
            finalText, 
            fmt.Sprintf(`$1"%s"`, secret),
        )
    }
    
    // 3. Валидируем финальный результат (обязательно)
    if !jsonc.Valid([]byte(finalText)) {
        return "", fmt.Errorf("invalid JSONC after secret replacement")
    }
    
    return finalText, nil
}
```

**Примечание:** 
- Замена `CHANGE_THIS_TO_YOUR_SECRET_TOKEN` - необязательная операция, просто текстовая замена константы
- Используем текстовую замену через regex, избегаем `json.MarshalIndent` для сохранения комментариев
- Если secret не найден - оставляем как есть (не создаем experimental.clash_api секцию)


## Части конфига, не требующие специальной обработки

### 1. Блок @ParserConfig

**Расположение:** В начале файла, в многострочном комментарии `/** @ParserConfig ... */`

**Обработка:** Блок заменяется полностью на новый ParserConfig из UI визарда.

**Проблем с комментариями нет:** Блок заменяется целиком, комментарии сохраняются в новом блоке.

**Текущая реализация:** Уже работает корректно в `BuildTemplateConfig`:
```go
builder.WriteString("/** @ParserConfig\n")
builder.WriteString(parserConfigText)
builder.WriteString("\n*/\n")
```

### 2. Поле "comment" в outbounds

**Расположение:** Внутри объектов outbounds как JSON поле: `"comment": "текст"`

**Важно:** Это **НЕ комментарий**, а обычное JSON поле. Сохраняется автоматически при парсинге/сериализации JSON.

**Пример:**
```json
{
  "tag": "auto-proxy-out",
  "type": "urltest",
  "comment": "Proxy automated group for everything that should go through VPN"
}
```

**Проблем с сохранением нет:** JSON поле сохраняется при любых операциях с JSON.

### 3. Блок @PARSER_OUTBOUNDS_BLOCK

**Расположение:** Внутри секции `"outbounds"`: `/** @PARSER_OUTBOUNDS_BLOCK */`

**Обработка:** Маркер заменяется текстово на сгенерированные outbounds из парсера.

**Проблем с комментариями нет:** Текстовая замена сохраняет комментарии вокруг маркера.

**Текущая реализация:** Уже работает в `BuildParserOutboundsBlock`:
```go
builder.WriteString(indent + "/** @ParserSTART */\n")
// ... сгенерированные outbounds ...
builder.WriteString(indent + "/** @ParserEND */")
```

**Вывод:** Эти части конфига не требуют специальной обработки для сохранения комментариев. Основная проблема - в секциях, которые форматируются через `json.Indent` или парсятся через `json.Unmarshal`/`json.Marshal`.

### 4. Trailing commas (запятые в конце списка)

**Расположение:** В конце массивов и объектов: `{ "key": "value", }` или `[item1, item2, ]`

**Поддержка JSONC:** JSONC (JSON with Comments) поддерживает trailing commas, так как это расширение JSON. Sing-box использует JSONC парсер, который должен поддерживать trailing commas.

**Обработка trailing commas:** Удаляем trailing comma только в местах, где мы вмешиваемся в текст:
1. **В rules массиве** - при добавлении/обработке правил
2. **В outbounds массиве** - при обработке @PARSER_OUTBOUNDS_BLOCK

В других местах trailing commas оставляем как есть (JSONC поддерживает).

**Обработка в route секции:** При обработке rules массива нужно удалять trailing comma перед закрывающей скобкой:

**Проблема:** После последнего правила в rules может быть запятая перед `]`: 
```json
    {"rule": "value"},
      ],
      "final": "proxy-out",
```

**Решение:** Использовать regex замену для удаления trailing comma:
```go
// Паттерн: ,\s*(],\s+"final": "...")
// Примечание: \s в Go regex включает \n (перевод строки), поэтому паттерн работает для многострочных случаев
trailingCommaPattern := regexp.MustCompile(`,\s*(\],\s+"final":\s*"[^"]*")`)
result := trailingCommaPattern.ReplaceAllString(routeText, `$1`)
```

**Пример замены:**
- **До:** `    {"rule": "value"},\n      ],\n      "final": "proxy-out"`
- **После:** `    {"rule": "value"}\n      ],\n      "final": "proxy-out"`

**Примечание:** Хотя JSONC поддерживает trailing commas, лучше их удалять для совместимости и чистоты конфига. Эта замена решает вопрос запятых в конце rules секции.


## План реализации

### Фаза 1: Модификация загрузки шаблона

**Задачи:**
1. Изменить `LoadTemplateData` в `template/loader.go`:
   - **Блоки остаются в тексте** (в отличие от текущей реализации, где они удаляются)
   - Сохранять исходный JSONC текст с блоками и комментариями
   - Извлекать метаданные из блоков для UI (как сейчас)

2. Обновить `TemplateData`:
   - Убедиться, что `Sections[key]` хранит исходный JSONC текст
   - Проверить, что метаданные извлекаются корректно

3. Создать тесты:
   - Проверка сохранения @SelectableRule блоков в тексте
   - Проверка извлечения метаданных
   - Валидация JSONC текста

**Файлы:**
- `ui/wizard/template/loader.go` - модификация загрузки
- `ui/wizard/template/loader_test.go` - тесты

### Фаза 2: Обработка @SelectableRule блоков

**Задачи:**
1. Создать `SelectableRuleTextProcessor`:
   - `ProcessRuleBlock` - обработка одного блока (замена блока на обработанную версию)
   - `RemoveDuplicateJSON` - удаление дублирующего JSON после блока
   - Логика замены блоков в зависимости от состояния правила (включено/выключено)

2. Реализовать обработку всех блоков:
   - Поиск всех @SelectableRule блоков в тексте
   - Обработка каждого блока
   - Сохранение комментариев

3. Создать тесты:
   - Тесты для включенных правил
   - Тесты для выключенных правил
   - Тесты для удаления дублирующего JSON
   - Edge cases

**Файлы:**
- `ui/wizard/business/selectable_rule_processor.go` - новый файл
- `ui/wizard/business/selectable_rule_processor_test.go` - тесты

### Фаза 3: Слияние route секции (текстовый подход)

**Задачи:**
1. Реализовать `MergeRouteSectionText`:
   - Обработка @SelectableRule блоков через текстовые замены
   - Добавление пользовательских правил (текстово)
   - Обновление final outbound (текстовая замена)
   - Валидация результата

2. Интегрировать в `BuildTemplateConfig`:
   - Заменить `MergeRouteSection` на `MergeRouteSectionText`
   - Использовать исходный JSONC текст вместо парсинга
   - Сохранять все комментарии

3. Создать тесты:
   - Тесты для обработки @SelectableRule блоков
   - Тесты для добавления пользовательских правил
   - Тесты для обновления final outbound
   - Проверка сохранения всех комментариев

**Файлы:**
- Модификация `ui/wizard/business/generator.go`
- `ui/wizard/business/route_text_merger.go` - новый файл
- `ui/wizard/business/route_text_merger_test.go` - тесты

### Фаза 4: Сохранение секций без форматирования

**Задачи:**
1. Модифицировать `BuildTemplateConfig`:
   - Для секций `log`, `dns`, `inbounds`, `experimental` - **НЕ использовать `FormatSectionJSON`**
   - Сохранять исходный JSONC текст как есть (или с минимальной проверкой валидности)
   - Применять только базовые отступы для вставки в финальный конфиг

2. Создать функцию `PreserveSectionText`:
   - Принимает исходный JSONC текст секции
   - Валидирует через `jsonc.Valid` (только проверка)
   - Возвращает текст с применением базовых отступов (если нужно)
   - **НЕ использует `json.Indent`**

3. Модифицировать `SaveConfigWithBackup`:
   - Использовать regex для замены secret (сохраняет комментарии)
   - Избегать `json.MarshalIndent` полностью
   - Сохранять исходный формат с комментариями

4. Создать тесты:
   - Тесты для сохранения секций без форматирования
   - Тесты для замены secret
   - Проверка сохранения всех комментариев во всех секциях
   - Edge cases

**Файлы:**
- Модификация `ui/wizard/business/generator.go`
- Модификация `ui/wizard/business/saver.go`
- Обновление тестов

**Примечание:** Для большинства секций (log, dns, inbounds, experimental) исходное форматирование уже корректное, поэтому можно просто сохранять текст как есть, без дополнительного форматирования.

### Фаза 5: Интеграционное тестирование

**Задачи:**
1. Тестирование на реальных шаблонах
   - `config_template.json`
   - `config_template_macos.json`
   - `config.example.json`

2. Проверка всех секций
   - log, dns, inbounds, outbounds, route, experimental
   - Проверка всех типов комментариев

3. Валидация финального конфига
   - Валидность JSONC
   - Корректность парсинга sing-box
   - Сохранение всех комментариев

**Файлы:**
- Интеграционные тесты
- Тесты на реальных данных

### Фаза 6: Оптимизация и рефакторинг

**Задачи:**
1. Оптимизация производительности
   - Профилирование
   - Улучшение алгоритмов

2. Рефакторинг кода
   - Улучшение читаемости
   - Удаление дублирования
   - Документация

**Файлы:**
- Рефакторинг всех измененных файлов

## Детальный разбор компонентов

### 1. Обработка @SelectableRule блоков

**Логика:** При генерации конфига каждый @SelectableRule блок **заменяется** на обработанную версию в зависимости от состояния правила (включено/выключено). Блоки не удаляются, а именно заменяются.

#### Алгоритм обработки одного блока

```go
func (p *SelectableRuleTextProcessor) ProcessRuleBlock(
    templateText string,
    ruleBlock string,
    ruleState *RuleState,
) (string, error) {
    // 1. Находим блок в тексте
    blockPattern := regexp.MustCompile(`(?is)(/\*\*\s*@selectablerule.*?\*/)`)
    matches := blockPattern.FindStringSubmatchIndex(templateText)
    if len(matches) == 0 {
        return templateText, nil // Блок не найден
    }
    
    // 2. Извлекаем JSON правило из блока (строка, начинающаяся с {)
    // Служебные данные начинаются с @, шаблон для вставки начинается с {
    jsonRule := extractJSONFromBlock(ruleBlock)
    
    // 3. Проверяем состояние правила и заменяем блок на обработанную версию
    if !ruleState.Enabled {
        // Правило выключено: заменяем блок на версию с @default, удаляем JSON после блока
        newBlock := addDefaultToBlock(ruleBlock)
        // Удаляем JSON правило сразу после закрывающего */
        textAfterBlock := templateText[matches[1]:]
        cleanedAfterBlock := removeJSONAfterBlock(textAfterBlock)
        // Заменяем исходный блок на обработанную версию
        return templateText[:matches[0]] + newBlock + cleanedAfterBlock, nil
    }
    
    // 4. Правило включено: заменяем блок на версию с JSON правилом после блока
    newBlock := ruleBlock // Блок остается как есть
    jsonRuleText := formatRuleJSON(ruleState.Rule, ruleState.Outbound) // json.Marshal с отступами 4 пробела
    
    // 5. Удаляем дублирующий JSON после блока (если есть)
    // Просто удаляем первый JSON сразу после закрывающего */
    textAfterBlock := templateText[matches[1]:]
    cleanedAfterBlock := removeJSONAfterBlock(textAfterBlock)
    
    // 6. Заменяем исходный блок на обработанную версию (блок + JSON правило)
    result := templateText[:matches[0]] + newBlock + "\n    " + jsonRuleText + cleanedAfterBlock
    
    return result, nil
}
```

#### Удаление JSON после блока

**Принцип:** НЕ сравнивать JSON. Просто удалить JSON правило, которое идет сразу после закрывающего `*/` блока.

**Пример:**
```json
  /**   @SelectableRule
        @label Block Ads
        {"rule_set": "ads-all", "action": "reject"},
  */
  {"rule_set": "ads-all", "action": "reject"},  // <-- JSON после блока, удаляем
```

**Алгоритм:**
```go
func removeJSONAfterBlock(
    text string,
) string {
    // Находим JSON правило сразу после закрывающего */ блока
    // Паттерн: после */ идёт JSON правило (начинается с {)
    jsonPattern := regexp.MustCompile(`(\s*\{[^}]*"rule_set"[^}]*\}[,\s]*)`)
    // Удаляем первый JSON после блока (не сравниваем структуру)
    return jsonPattern.ReplaceAllString(text, "", 1) // Заменяем только первое вхождение
}
```

### 2. Обработка секций (log, dns, inbounds, experimental)

**Принцип:** Возвращаем текст как есть, без форматирования. Меняем только то, что нужно изменить (/**...*/ блоки).

```go
func ProcessSection(
    raw json.RawMessage, // Исходный JSONC текст
) (string, error) {
    text := string(raw)
    
    // 1. Валидируем JSONC (только для проверки)
    if !jsonc.Valid([]byte(text)) {
        return "", fmt.Errorf("invalid JSONC")
    }
    
    // 2. Возвращаем текст как есть - сохраняем все комментарии и форматирование
    return text, nil
}
```

**Примечание:** Для секций `log`, `dns`, `inbounds`, `experimental` не требуется никакого форматирования. Просто возвращаем исходный текст из шаблона.

### 3. Слияние route секции (текстовый подход)

#### Алгоритм слияния

```go
func MergeRouteSectionText(
    templateText string,        // Исходный JSONC текст route секции
    ruleStates []*RuleState,    // Состояния правил из UI
    customRules []*RuleState,   // Пользовательские правила
    finalOutbound string,       // Финальный outbound
) (string, error) {
    result := templateText
    
    // 1. Обрабатываем все @SelectableRule блоки
    processor := &SelectableRuleTextProcessor{}
    for _, ruleState := range ruleStates {
        // Находим соответствующий блок в тексте
        block := findSelectableRuleBlock(result, ruleState)
        if block != "" {
            var err error
            result, err = processor.ProcessRuleBlock(result, block, ruleState)
            if err != nil {
                return "", fmt.Errorf("failed to process rule block: %w", err)
            }
        }
    }
    
    // 2. Добавляем пользовательские правила (текстово, после существующих)
    if len(customRules) > 0 {
        result = addCustomRulesText(result, customRules)
    }
    
    // 3. Удаляем trailing comma перед закрывающей скобкой rules (если есть)
    // Паттерн: ,\s*(],\s+"final": "...")
    // Примечание: \s в Go regex включает \n (перевод строки), поэтому паттерн работает для многострочных случаев
    trailingCommaPattern := regexp.MustCompile(`,\s*(\],\s+"final":\s*"[^"]*")`)
    result = trailingCommaPattern.ReplaceAllString(result, `$1`)
    
    // 4. Обновляем final outbound (текстовая замена)
    if finalOutbound != "" {
        result = updateFinalOutboundText(result, finalOutbound)
    }
    
    // 5. Валидируем результат (обязательно)
    // Валидируем после критических операций и обязательно финальный файл
    if !jsonc.Valid([]byte(result)) {
        return "", fmt.Errorf("invalid JSONC after merge")
    }
    
    return result, nil
}
```

#### Добавление пользовательских правил

```go
func addCustomRulesText(
    routeText string,
    customRules []*RuleState,
) string {
    // Находим позицию перед закрывающей скобкой rules массива
    rulesEndPattern := regexp.MustCompile(`(\s*)(\])(\s*"final")`)
    
    rulesText := ""
    for _, rule := range customRules {
        if rule.Enabled {
            ruleJSON := formatRuleJSON(rule.Rule, rule.Outbound)
            rulesText += "    " + ruleJSON + ",\n"
        }
    }
    
    if rulesText != "" {
        // Вставляем правила перед закрывающей скобкой
        replacement := "$1" + rulesText + "$1$2$3"
        result := rulesEndPattern.ReplaceAllString(routeText, replacement)
        
        // Удаляем trailing comma перед закрывающей скобкой rules (если есть)
        // Паттерн: ,\s*(],\s+"final": "...")
        // Примечание: \s в Go regex включает \n (перевод строки), поэтому паттерн работает для многострочных случаев
        trailingCommaPattern := regexp.MustCompile(`,\s*(\],\s+"final":\s*"[^"]*")`)
        result = trailingCommaPattern.ReplaceAllString(result, `$1`)
        
        return result
    }
    
    // Даже если не добавляли правила, нужно проверить trailing comma
    // Примечание: \s в Go regex включает \n (перевод строки), поэтому паттерн работает для многострочных случаев
    trailingCommaPattern := regexp.MustCompile(`,\s*(\],\s+"final":\s*"[^"]*")`)
    return trailingCommaPattern.ReplaceAllString(routeText, `$1`)
}
```

**Примечание:** Regex `,\s*(\],\s+"final":\s*"[^"]*")` находит запятую перед закрывающей скобкой `]` в rules массиве и удаляет её, заменяя на `$1` (только скобку и "final"). Это решает проблему trailing comma в конце rules секции.

#### Обновление final outbound

```go
func updateFinalOutboundText(
    routeText string,
    finalOutbound string,
) string {
    // Ищем "final": "..." и заменяем
    finalPattern := regexp.MustCompile(`("final"\s*:\s*)"[^"]*"`)
    if finalPattern.MatchString(routeText) {
        return finalPattern.ReplaceAllString(
            routeText,
            fmt.Sprintf(`$1"%s"`, finalOutbound),
        )
    }
    
    // Если "final" не найден, добавляем его
    // (находим закрывающую скобку route и добавляем перед ней)
    routeEndPattern := regexp.MustCompile(`(\s*)(\})`)
    addition := fmt.Sprintf(`$1  "final": "%s",$1$2`, finalOutbound)
    return routeEndPattern.ReplaceAllString(routeText, addition)
}
```

## Риски и митигация

### Риск 1: Ошибки при текстовых заменах

**Описание:** Неправильные regex или текстовые замены могут испортить структуру JSONC.

**Митигация:**
- Тщательное тестирование всех случаев замен
- Валидация результата через `jsonc.Valid` после каждой операции
- Fallback на исходный текст при ошибках
- Тестирование на реальных шаблонах

### Риск 2: Неправильное определение @SelectableRule блоков

**Описание:** Regex может неправильно найти или обработать блоки.

**Митигация:**
- Тщательное тестирование всех форматов блоков
- Проверка границ блоков (начало/конец)
- Валидация структуры блока перед обработкой
- Логирование для отладки

### Риск 3: Проблемы с удалением JSON после блока

**Описание:** Алгоритм удаления JSON после блока может удалить неправильные части.

**Митигация:**
- Удалять только первый JSON сразу после закрывающего `*/` блока
- Тестирование всех возможных форматов правил
- Сохранение исходного текста для отката
- Валидация после удаления

### Риск 4: Несовместимость с sing-box

**Описание:** Финальный конфиг может быть невалидным для sing-box.

**Митигация:**
- **Обязательная валидация** финального собранного файла через `jsonc.Valid` после всех операций
- Тестирование с реальным sing-box
- Проверка парсинга sing-box
- Сохранение исходного текста для сравнения

### Риск 5: Производительность при больших конфигах

**Описание:** Множественные текстовые операции могут замедлить процесс.

**Митигация:**
- Оптимизация regex (компиляция один раз)
- Минимизация копирований строк
- Профилирование и оптимизация горячих мест
- Кэширование результатов поиска блоков

## Критерии успеха

### Функциональные критерии

1. ✅ Все комментарии из шаблона сохраняются в финальном конфиге
2. ✅ Комментарии находятся в правильных позициях
3. ✅ Конфиг остается валидным JSONC
4. ✅ Sing-box корректно парсит финальный конфиг
5. ✅ Все секции обрабатываются корректно

### Технические критерии

1. ✅ Производительность не ухудшается более чем на 20%
2. ✅ Код покрыт тестами (минимум 80%)
3. ✅ Обратная совместимость сохранена
4. ✅ Нет регрессий в существующем функционале

### Пользовательские критерии

1. ✅ Пользователи могут использовать конфиги как шаблоны
2. ✅ Комментарии помогают понимать конфигурацию
3. ✅ Конфиг остается читаемым и документированным


