# Оставшиеся задачи и оптимизации

## Статус выполненных рефакторингов

### ✅ Выполнено

1. **Рефакторинг UI визарда** - создана модульная структура `ui/wizard/` с архитектурой MVP (Model-View-Presenter)
2. **Рефакторинг структуры конфигурации** - создана структура `core/config/` с разделением ответственности
3. **Рефакторинг AppController** - выделены сервисы: `UIService`, `APIService`, `StateService`, `FileService`
4. **Централизация логирования** - используется `internal/debuglog` с глобальным уровнем через `SINGBOX_DEBUG`
5. **Консолидация валидации** - реализована `ValidateStringLength()` и используется в валидаторах
6. **Документация** - создан `ARCHITECTURE.md` с детальным описанием структуры

---

## Актуальные задачи оптимизации

### Приоритет 1: Упрощение кода (СРЕДНИЙ приоритет)

#### Задача 1.1: Удобные функции логирования в debuglog

**Текущая ситуация:**
- В `internal/debuglog` есть только функция `Log()` с 5 параметрами
- Во всем проекте используется паттерн: `debuglog.Log("DEBUG", debuglog.LevelVerbose, debuglog.UseGlobal, format, args...)`
- В `ui/wizard/business/parser.go` более 20 таких вызовов, что ухудшает читаемость
- Аналогичные вызовы есть и в других частях проекта (`core/`, `ui/wizard/`)

**Обоснование:**
- Упростит код: вместо 5 параметров будет 2 (`debuglog.DebugLog(format, args...)`)
- Улучшит читаемость: явные имена функций (`DebugLog`, `InfoLog`, `ErrorLog`) понятнее, чем магические константы
- Уменьшит вероятность ошибок: не нужно помнить порядок параметров `debuglog.Log`
- Глобальная доступность: функции будут доступны во всем проекте, а не только в wizard
- Не нарушает существующую функциональность: глобальный уровень логирования уже контролируется через `SINGBOX_DEBUG`

**Что сделать:**
1. Добавить удобные функции в `internal/debuglog/debuglog.go`:
   ```go
   // DebugLog logs a debug message (LevelVerbose) with "DEBUG" prefix
   func DebugLog(format string, args ...interface{}) {
       Log("DEBUG", LevelVerbose, UseGlobal, format, args...)
   }
   
   // InfoLog logs an info message (LevelInfo) with "INFO" prefix
   func InfoLog(format string, args ...interface{}) {
       Log("INFO", LevelInfo, UseGlobal, format, args...)
   }
   
   // ErrorLog logs an error message (LevelError) with "ERROR" prefix
   func ErrorLog(format string, args ...interface{}) {
       Log("ERROR", LevelError, UseGlobal, format, args...)
   }
   ```
2. Заменить вызовы `debuglog.Log()` на новые функции в:
   - `ui/wizard/business/parser.go`
   - `ui/wizard/business/generator.go`
   - `ui/wizard/business/loader.go`
   - `ui/wizard/business/saver.go`
   - `ui/wizard/wizard.go`
   - Другие файлы, где используется длинный паттерн

**Ожидаемый результат:**
- Упрощение ~30-40 вызовов логирования в wizard
- Улучшение читаемости кода во всем проекте
- Единообразный стиль логирования
- Расширение глобального механизма логирования, а не создание дублирующего функционала

**Файлы для изменения:**
- `internal/debuglog/debuglog.go` (добавить функции)
- `ui/wizard/business/parser.go`
- `ui/wizard/business/generator.go`
- `ui/wizard/business/loader.go`
- `ui/wizard/business/saver.go`
- `ui/wizard/wizard.go`

---

#### Задача 1.2: Упрощение SafeFyneDo через метод UpdateUI

**Текущая ситуация:**
- В `presentation/` слое используется `SafeFyneDo(p.guiState.Window, func() { ... })` напрямую
- Более 20 таких вызовов в разных файлах презентера
- Дублирование `p.guiState.Window` в каждом вызове

**Обоснование:**
- Инкапсуляция: `p.guiState.Window` должен быть скрыт внутри презентера
- Упрощение: `p.UpdateUI(fn)` короче и понятнее, чем `SafeFyneDo(p.guiState.Window, fn)`
- Консистентность: все методы презентера будут использовать единый способ обновления UI
- Меньше ошибок: не нужно помнить передавать `p.guiState.Window` каждый раз

**Что сделать:**
1. Добавить метод `UpdateUI(fn func())` в `WizardPresenter`:
   ```go
   func (p *WizardPresenter) UpdateUI(fn func()) {
       SafeFyneDo(p.guiState.Window, fn)
   }
   ```
2. Заменить прямые вызовы `SafeFyneDo(p.guiState.Window, ...)` на `p.UpdateUI(...)`

**Ожидаемый результат:**
- Упрощение ~20 вызовов
- Улучшение инкапсуляции: доступ к `guiState.Window` только через метод презентера
- Более чистый и читаемый код

**Файлы для изменения:**
- `ui/wizard/presentation/presenter.go` (добавить метод)
- `ui/wizard/presentation/presenter_methods.go`
- `ui/wizard/presentation/presenter_save.go`
- `ui/wizard/presentation/presenter_sync.go`
- `ui/wizard/presentation/presenter_ui_updater.go`

---

### Приоритет 2: Утилиты для повторяющихся паттернов (СРЕДНИЙ приоритет)

#### Задача 2.1: Утилита для измерения времени выполнения

**Текущая ситуация:**
- В `business/parser.go` и `business/generator.go` используется паттерн:
  ```go
  startTime := time.Now()
  debuglog.Log("DEBUG", debuglog.LevelVerbose, debuglog.UseGlobal, "funcName: START at %s", startTime.Format("15:04:05.000"))
  // ... код ...
  debuglog.Log("DEBUG", debuglog.LevelVerbose, debuglog.UseGlobal, "funcName: END (total duration: %v)", time.Since(startTime))
  ```
- Повторяется в 5-6 местах
- Форматирование времени и логирование дублируется

**Обоснование:**
- DRY принцип: убрать дублирование кода измерения времени
- Упрощение: вместо 3-4 строк достаточно 2 (`timing := debuglog.StartTiming("funcName")` + `defer timing.EndWithDefer()`)
- Единообразие: все измерения времени будут в одном стиле
- Расширяемость: легко добавить промежуточные измерения через `LogTiming()`
- Логичное размещение: утилита тесно связана с логированием и использует `debuglog`, поэтому должна быть в пакете `debuglog`

**Что сделать:**
1. Добавить в `internal/debuglog/debuglog.go`:
   ```go
   // TimingContext tracks timing for a function execution
   type TimingContext struct {
       startTime time.Time
       funcName  string
   }
   
   // StartTiming creates a new timing context and logs start
   func StartTiming(funcName string) *TimingContext {
       startTime := time.Now()
       DebugLog("%s: START at %s", funcName, startTime.Format("15:04:05.000"))
       return &TimingContext{
           startTime: startTime,
           funcName: funcName,
       }
   }
   
   // End logs total duration and returns it
   func (tc *TimingContext) End() time.Duration {
       duration := time.Since(tc.startTime)
       DebugLog("%s: END (total duration: %v)", tc.funcName, duration)
       return duration
   }
   
   // EndWithDefer returns a defer function for automatic logging
   func (tc *TimingContext) EndWithDefer() func() {
       return func() {
           tc.End()
       }
   }
   
   // LogTiming logs elapsed time for a specific operation
   func (tc *TimingContext) LogTiming(operation string, duration time.Duration) {
       DebugLog("%s: %s took %v", tc.funcName, operation, duration)
   }
   ```
2. Заменить паттерны в `business/parser.go` и `business/generator.go`

**Ожидаемый результат:**
- Упрощение ~15-20 строк кода
- Единообразный стиль измерения времени
- Легче добавлять промежуточные измерения
- Утилита доступна во всем проекте через `debuglog`

**Файлы для изменения:**
- `internal/debuglog/debuglog.go` (добавить TimingContext и методы)
- `ui/wizard/business/parser.go`
- `ui/wizard/business/generator.go`

---

### Приоритет 3: Мелкие улучшения (НИЗКИЙ приоритет)

#### Задача 3.1: Устранение дублирования констант

**Текущая ситуация:**
- В `ui/wizard/business/validator.go` есть TODO:
  ```go
  // TODO: Устранить дублирование - использовать parser.MaxConfigFileSize из core/config/parser
  // вместо wizardutils.MaxJSONConfigSize (оба имеют одинаковое значение 50MB).
  ```
- Две константы с одинаковым значением в разных пакетах

**Обоснование:**
- Устранение дублирования: одна константа вместо двух
- Консистентность: изменения лимита в одном месте
- НО: нужно проверить, не нарушит ли это разделение ответственности (wizard utils vs core config)

**Что сделать:**
1. Проверить, можно ли использовать `parser.MaxConfigFileSize` вместо `wizardutils.MaxJSONConfigSize`
2. Убедиться, что это не создаст циклических зависимостей
3. Если безопасно - заменить использование

**Ожидаемый результат:**
- Устранение дублирования
- Одна точка истины для лимита размера конфигурации

**Файлы для изменения:**
- `ui/wizard/business/validator.go`
- Возможно `ui/wizard/utils/constants.go`

---

## Рекомендуемый порядок выполнения

### Этап 1: Упрощение кода (1-2 дня)
1. Задача 1.1: Wrapper-функции для логирования
2. Задача 1.2: Упрощение SafeFyneDo

**Ожидаемая экономия:** ~50-60 строк, значительное улучшение читаемости

### Этап 2: Утилиты (1 день)
3. Задача 2.1: Утилита для таймингов

**Ожидаемая экономия:** ~15-20 строк, единообразие кода

### Этап 3: Мелкие улучшения (по необходимости)
4. Задача 3.1: Устранение дублирования констант

**Ожидаемая экономия:** ~5-10 строк, улучшение консистентности

---

## Итоговая статистика

| Задача | Приоритет | Ожидаемая экономия | Обоснование |
|--------|-----------|---------------------|-------------|
| Wrapper-функции для логирования | СРЕДНИЙ | ~30-40 упрощений | Улучшение читаемости, единообразие |
| Упрощение SafeFyneDo | СРЕДНИЙ | ~20 упрощений | Инкапсуляция, читаемость |
| Утилита для таймингов | СРЕДНИЙ | ~15-20 строк | DRY, единообразие |
| Устранение дублирования констант | НИЗКИЙ | ~5-10 строк | Консистентность |

---

## Примечания

- Все оптимизации должны проходить `go vet` и `go test`
- Не жертвовать читаемостью ради экономии строк
- Сохранять обратную совместимость API
- Измерять реальную экономию после каждой оптимизации
- Архитектура MVP (WizardPresenter + GUIState) должна сохраняться

**Важно:** Эти оптимизации направлены на улучшение читаемости и поддержки кода, а не на радикальное сокращение объема. Основная цель - сделать код более понятным и единообразным.
