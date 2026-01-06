# Покрытие тестами визарда конфигурации

## Обзор

Все тесты переписаны под новую архитектуру MVP и проверяют бизнес-логику без зависимостей от GUI.

## business/generator_test.go

### TestMergeRouteSection
**Проверяет:** Объединение правил маршрутизации из шаблона и пользовательских правил
- Объединение правил из шаблона (rawRoute) с правилами из `SelectableRuleStates`
- Объединение с пользовательскими правилами (`CustomRules`)
- Корректность установки `final` outbound в результате
- Проверка, что все правила присутствуют в итоговом JSON (исходное + selectable + custom)

### TestMergeRouteSection_RejectAction
**Проверяет:** Обработку действия "reject" для правил
- Правило с `SelectedOutbound = "reject"` преобразуется в правило с `action: "reject"`
- Поле `outbound` удаляется из правила при использовании reject
- Правило корректно включается в результат

### TestMergeRouteSection_DisabledRules
**Проверяет:** Исключение отключенных правил из результата
- Правила с `Enabled = false` не включаются в финальную конфигурацию
- Включены только правила с `Enabled = true`

### TestFormatSectionJSON
**Проверяет:** Форматирование JSON секций с отступами
- Корректное форматирование валидного JSON с разными уровнями отступов (2, 4)
- Обработка ошибок при невалидном JSON
- Сохранение содержимого JSON при форматировании

### TestIndentMultiline
**Проверяет:** Добавление отступов к многострочному тексту
- Добавление отступов к одной строке
- Добавление отступов ко всем строкам в многострочном тексте
- Обработка пустого текста
- Обработка текста с завершающим переносом строки

## business/parser_test.go

### TestSerializeParserConfig_Standalone
**Проверяет:** Сериализацию ParserConfig в JSON
- Корректная сериализация валидного ParserConfig в JSON строку
- Результат является валидным JSON
- Результат содержит блок `ParserConfig`
- Обработка ошибок при `nil` ParserConfig

### TestApplyURLToParserConfig_Logic
**Проверяет:** Логику классификации URL и прямых ссылок
- Разделение входных строк на подписки (http://, https://) и прямые ссылки (vless://, vmess://)
- Корректный подсчет количества подписок и прямых ссылок
- Пропуск пустых строк

## business/loader_test.go

### TestSerializeParserConfig
**Проверяет:** Сериализацию ParserConfig (дубликат из parser_test.go для loader пакета)
- Аналогично `TestSerializeParserConfig_Standalone`

### TestCloneOutbound
**Проверяет:** Глубокое клонирование OutboundConfig
- Клон является отдельным экземпляром (не указатель на исходный)
- Все поля скопированы корректно (Tag, Type, Comment, AddOutbounds)
- Глубокое копирование вложенных структур (Options, Filters)
- Изменение исходного объекта не влияет на клон (проверка на AddOutbounds и Options)

### TestEnsureRequiredOutbounds
**Проверяет:** Добавление обязательных outbounds из шаблона
- Outbounds с `wizard.required = 1` добавляются в конфигурацию (если отсутствуют)
- Outbounds с `wizard.required = 2` добавляются/перезаписываются в конфигурации
- Outbounds с `wizard.required = 0` (опциональные) не добавляются автоматически
- Корректность типов добавленных outbounds

### TestEnsureRequiredOutbounds_Overwrite
**Проверяет:** Перезапись обязательных outbounds с `required > 1`
- Существующие outbounds с `required = 2` перезаписываются из шаблона
- Тип outbound изменяется на значение из шаблона

### TestEnsureRequiredOutbounds_Preserve
**Проверяет:** Сохранение обязательных outbounds с `required = 1`
- Существующие outbounds с `required = 1` не перезаписываются
- Тип outbound сохраняется из существующей конфигурации

### TestLoadConfigFromFile_FileSizeValidation
**Проверяет:** Валидацию размера файла конфигурации
- Создание временного файла, превышающего лимит размера
- Проверка, что файл действительно превышает `MaxJSONConfigSize`

## business/validator_test.go

### TestValidateParserConfig
**Проверяет:** Валидацию структуры ParserConfig
- Валидация валидного ParserConfig проходит успешно
- Ошибка при `nil` ParserConfig
- Ошибка при `nil` Proxies в ParserConfig
- Ошибка при невалидных URL в Proxies
- Ошибка при невалидных URI в Connections
- Ошибка при невалидных outbounds (пустой Tag)

### TestValidateURL
**Проверяет:** Валидацию URL подписок
- Валидация валидных HTTP/HTTPS URL
- Ошибка при пустом URL
- Ошибка при URL превышающем максимальную длину
- Ошибка при слишком коротком URL
- Ошибка при URL без схемы (http://, https://)
- Ошибка при URL без хоста
- Ошибка при невалидном формате URL

### TestValidateURI
**Проверяет:** Валидацию URI прямых ссылок
- Валидация валидных URI (vless://, vmess://, trojan://)
- Ошибка при пустом URI
- Ошибка при URI превышающем максимальную длину
- Ошибка при слишком коротком URI
- Ошибка при URI без протокола (://)
- Ошибка при невалидном формате URI

### TestValidateOutbound
**Проверяет:** Валидацию конфигурации outbound
- Валидация валидного outbound
- Ошибка при `nil` outbound
- Ошибка при пустом Tag
- Ошибка при пустом Type
- Ошибка при Tag превышающем максимальную длину (256 символов)

### TestValidateJSONSize
**Проверяет:** Валидацию размера JSON данных
- Валидация данных допустимого размера
- Валидация данных точно на лимите `MaxJSONConfigSize`
- Ошибка при данных превышающих лимит
- Валидация пустых данных (разрешены)

### TestValidateJSON
**Проверяет:** Валидацию JSON структуры
- Валидация валидного JSON
- Ошибка при невалидном JSON (незакрытые скобки)
- Ошибка при JSON превышающем размер `MaxJSONConfigSize`
- Валидация пустого JSON объекта `{}`

### TestValidateHTTPResponseSize
**Проверяет:** Валидацию размера HTTP ответа
- Валидация размера в пределах лимита
- Валидация размера точно на лимите `MaxSubscriptionSize`
- Ошибка при размере превышающем лимит
- Валидация нулевого размера (разрешен)

### TestValidateParserConfigJSON
**Проверяет:** Валидацию JSON текста ParserConfig
- Валидация валидного JSON ParserConfig
- Ошибка при пустом JSON
- Ошибка при невалидном JSON синтаксисе
- Ошибка при JSON превышающем размер `MaxJSONConfigSize`
- Ошибка при невалидной структуре ParserConfig (nil proxies)

### TestValidateRule
**Проверяет:** Валидацию правил маршрутизации
- Валидация правила с полем `domain`
- Валидация правила с полем `ip_cidr`
- Ошибка при `nil` правиле
- Ошибка при пустом правиле (нет полей)

## business/outbound_test.go

### TestGetAvailableOutbounds
**Проверяет:** Получение списка доступных outbound тегов из модели
- Извлечение outbounds из `ParserConfig` объекта в модели
- Извлечение outbounds из `ParserConfigJSON` строки в модели
- Всегда включаются дефолтные outbounds: `direct-out`, `reject`, `drop`
- Включение outbounds из глобальных outbounds конфигурации
- Корректность минимального количества outbounds в результате
- Все ожидаемые теги присутствуют в результате

### TestEnsureDefaultAvailableOutbounds
**Проверяет:** Обеспечение наличия дефолтных outbounds
- При пустом списке возвращаются дефолтные: `["direct-out", "reject"]`
- При непустом списке исходный список сохраняется без изменений

### TestEnsureFinalSelected
**Проверяет:** Обеспечение выбранного final outbound в модели
- Если `SelectedFinalOutbound` уже установлен и присутствует в опциях - сохраняется
- Если `SelectedFinalOutbound` не установлен - используется `direct-out`
- Если `SelectedFinalOutbound` не в опциях - используется первый доступный вариант (`direct-out` приоритетно)
- Fallback логика при отсутствии предпочтительного outbound в опциях

## models/rule_state_utils_test.go

### TestGetEffectiveOutbound
**Проверяет:** Получение эффективного outbound для правила
- Если `SelectedOutbound` установлен - возвращается он
- Если `SelectedOutbound` пуст, но есть `DefaultOutbound` в Rule - возвращается `DefaultOutbound`
- Если оба пусты - возвращается пустая строка

### TestEnsureDefaultOutbound
**Проверяет:** Установку дефолтного outbound для правила
- Если `SelectedOutbound` уже установлен - не изменяется
- Если `SelectedOutbound` пуст и есть `DefaultOutbound` в Rule - устанавливается `DefaultOutbound`
- Если `SelectedOutbound` пуст и нет `DefaultOutbound`, но есть доступные outbounds - устанавливается первый доступный
- Если нет ни `DefaultOutbound`, ни доступных outbounds - остается пустым

## Итого

**Общее покрытие:**
- ✅ Генерация конфигурации (объединение правил, форматирование)
- ✅ Парсинг и сериализация ParserConfig
- ✅ Загрузка конфигурации (клонирование, добавление обязательных outbounds)
- ✅ Валидация (ParserConfig, URL, URI, Outbound, JSON, Rule)
- ✅ Работа с outbounds (получение списка, обеспечение дефолтных, выбор final)
- ✅ Работа с правилами (эффективный outbound, установка дефолтного)

**Все тесты:**
- Работают с чистыми данными (без GUI зависимостей)
- Используют новую архитектуру MVP (`wizardmodels` вместо `wizardstate`)
- Легко выполняются без инициализации Fyne
- Покрывают как успешные сценарии, так и обработку ошибок

