# План рефакторинга визарда: Разделение GUI и бизнес-логики

## Текущие проблемы

### 1. Смешение GUI и бизнес-логики в `WizardState`

Файл `ui/wizard/state/state.go` содержит смешанные зависимости:

**GUI зависимости (Fyne):**
- `Window fyne.Window`
- `SourceURLEntry *widget.Entry`
- `URLStatusLabel *widget.Label`
- `ParserConfigEntry *widget.Entry`
- `OutboundsPreview *widget.Entry`
- `CheckURLButton *widget.Button`
- `CheckURLProgress *widget.ProgressBar`
- `ParseButton *widget.Button`
- `TemplatePreviewEntry *widget.Entry`
- `ShowPreviewButton *widget.Button`
- `FinalOutboundSelect *widget.Select`
- `CloseButton, PrevButton, NextButton, SaveButton *widget.Button`
- `SaveProgress *widget.ProgressBar`
- `Tabs *container.AppTabs`
- `CheckURLContainer, ButtonsContainer fyne.CanvasObject`
- `OpenRuleDialogs map[int]fyne.Window`

**Бизнес-данные:**
- `ParserConfig *config.ParserConfig`
- `GeneratedOutbounds []string`
- `TemplateData *wizardtemplate.TemplateData`
- `TemplateSectionSelections map[string]bool`
- `SelectableRuleStates []*SelectableRuleState`
- `CustomRules []*SelectableRuleState`
- `SelectedFinalOutbound string`
- `OutboundStats struct`

### 2. Бизнес-логика зависит от GUI

Все функции в `ui/wizard/business/` принимают `*WizardState`, который содержит Fyne-виджеты:
- `BuildTemplateConfig(state *WizardState, ...)`
- `CheckURL(state *WizardState)`
- `ParseAndPreview(state *WizardState)`
- `LoadConfigFromFile(state *WizardState)`

Это делает невозможным тестирование бизнес-логики без инициализации Fyne GUI.

### 3. GUI код напрямую обращается к бизнес-данным

В табах (`ui/wizard/tabs/`) код напрямую читает и изменяет бизнес-данные из `WizardState`, например:
- `state.ParserConfigEntry.Text`
- `state.SelectableRuleStates[idx].Enabled`
- `state.TemplateSectionSelections[key]`

## Цели рефакторинга

**Главная цель:** Создать управляемый и чистый код с четким разделением ответственности.

1. **Разделить модели данных и GUI-состояние**
   - Создать чистые модели данных без зависимостей от Fyne
   - Выделить GUI-состояние в отдельную структуру

2. **Сделать бизнес-логику тестируемой**
   - Бизнес-логика должна работать только с моделями данных
   - Убрать зависимости от Fyne из бизнес-логики

3. **Ввести слой представления (Presentation Layer)**
   - Создать адаптеры/презентеры для связи GUI и бизнес-логики
   - GUI обновляется через презентеры, а не напрямую

## Принципы рефакторинга

**ВАЖНО:** Рефакторинг выполняется БЕЗ требований на обратную совместимость API.

### Подход к рефакторингу

1. **Чистый код с первого раза**
   - Код пишется заново, а не мигрируется через обертки
   - Сразу применяются лучшие практики и паттерны
   - Учитывается имеющийся опыт разработки

2. **Удаление всего лишнего**
   - **Удаляются:** старые функции, обертки, дубли кода, артефакты из прошлого
   - **Удаляются:** лишние тесты (старые тесты, которые тестируют устаревшую архитектуру)
   - **Удаляется:** весь технический долг, связанный со старой архитектурой
   - **Удаляется:** неиспользуемый код, мертвый код, комментарии о "временных решениях"

3. **Оптимизация с учетом опыта**
   - Новый код оптимизируется на основе понимания требований
   - Избегаются известные проблемы из текущей реализации
   - Применяются решения, которые уже зарекомендовали себя

4. **Сохранение только функциональности**
   - Требуется сохранить только **поведение GUI** и **функциональность** визарда
   - Можно полностью переписать код, если это улучшает архитектуру
   - НЕ нужно сохранять старые функции, интерфейсы, структуры
   - НЕ нужно создавать функции-обертки для старого кода

## Архитектура после рефакторинга

```
┌─────────────────────────────────────────────────────────────┐
│                      Presentation Layer                      │
│  (ui/wizard/presentation/)                                   │
│  - WizardPresenter                                           │
│  - SourceTabPresenter                                        │
│  - RulesTabPresenter                                         │
│  - PreviewTabPresenter                                       │
│  - GUIState (только Fyne виджеты)                            │
└─────────────────────────────────────────────────────────────┘
                            │
                            │ использует
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Business Logic Layer                      │
│  (ui/wizard/business/)                                       │
│  - generator.go (BuildTemplateConfig, BuildParserOutboundsBlock)│
│  - parser.go (CheckURL, ParseAndPreview, ApplyURLToParserConfig)│
│  - loader.go (LoadConfigFromFile, EnsureRequiredOutbounds)  │
│  - validator.go (ValidateParserConfig, ValidateURL, ValidateURI)│
└─────────────────────────────────────────────────────────────┘
                            │
                            │ работает с
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      Domain Models                           │
│  (ui/wizard/models/)                                         │
│  - WizardModel (чистые данные)                               │
│  - RuleState (без GUI)                                       │
│  - TemplateState                                             │
└─────────────────────────────────────────────────────────────┘
```

## Детальный план рефакторинга

### Этап 1: Создание моделей данных

**Файлы для создания:**
- `ui/wizard/models/wizard_model.go`
- `ui/wizard/models/rule_state.go`

**Задачи:**
1. Создать структуру `WizardModel` с бизнес-данными (без Fyne зависимостей):
   ```go
   type WizardModel struct {
       // ParserConfig данные
       ParserConfigJSON string
       ParserConfig     *config.ParserConfig
       
       // Источники
       SourceURLs string
       
       // Сгенерированные outbounds
       GeneratedOutbounds []string
       OutboundStats      OutboundStats
       
       // Template данные
       TemplateData              *wizardtemplate.TemplateData
       TemplateSectionSelections map[string]bool
       
       // Правила
       SelectableRuleStates []*RuleState
       CustomRules          []*RuleState
       SelectedFinalOutbound string
       
       // Флаги состояния
       PreviewNeedsParse          bool
       TemplatePreviewNeedsUpdate bool
       AutoParseInProgress        bool
   }
   
   type RuleState struct {
       Rule             wizardtemplate.TemplateSelectableRule
       Enabled          bool
       SelectedOutbound string
       // НЕТ OutboundSelect *widget.Select!
   }
   ```

2. Создать структуру `GUIState` только для GUI-виджетов:
   ```go
   type GUIState struct {
       Window fyne.Window
       
       // Source tab widgets
       SourceURLEntry       *widget.Entry
       URLStatusLabel       *widget.Label
       ParserConfigEntry    *widget.Entry
       OutboundsPreview     *widget.Entry
       CheckURLButton       *widget.Button
       CheckURLProgress     *widget.ProgressBar
       ParseButton          *widget.Button
       
       // Rules tab widgets
       FinalOutboundSelect *widget.Select
       // ... другие виджеты
       
       // Navigation
       CloseButton *widget.Button
       PrevButton  *widget.Button
       NextButton  *widget.Button
       SaveButton  *widget.Button
       SaveProgress *widget.ProgressBar
       
       Tabs *container.AppTabs
   }
   ```

3. **НЕ создавать функции конвертации** - рефакторинг выполняется без обратной совместимости.
   
   Поскольку рефакторинг делается без требования обратной совместимости API,
   можно сразу переписывать код на новую архитектуру:
   - Создать `WizardModel` и `GUIState`
   - Переписать код сразу на новую архитектуру
   - Удалить `WizardState` после переписывания всего кода
   
   Функции конвертации нужны только для постепенной миграции с обратной совместимостью,
   но мы делаем полную переписку, поэтому они не нужны.

### Этап 2: Рефакторинг бизнес-логики

**Файлы для изменения:**
- `ui/wizard/business/generator.go`
- `ui/wizard/business/parser.go`
- `ui/wizard/business/loader.go`
- `ui/wizard/business/validator.go`

**Задачи:**
1. Изменить сигнатуры функций бизнес-логики, чтобы они принимали `*WizardModel` вместо `*WizardState`:
   ```go
   // Было:
   func BuildTemplateConfig(state *WizardState, forPreview bool) (string, error)
   
   // Станет:
   func BuildTemplateConfig(model *WizardModel, forPreview bool) (string, error)
   ```

2. Для функций, которые должны обновлять GUI, создать интерфейсы:
   ```go
   type UIUpdater interface {
       UpdateURLStatus(status string)
       UpdateProgress(progress float64)
       UpdatePreviewText(text string)
       // ... другие методы обновления UI
   }
   
   func CheckURL(model *WizardModel, updater UIUpdater) error
   ```

3. **НЕ создавать функции-обертки** - старые функции будут полностью заменены новыми

### Этап 3: Создание Presentation Layer

**Файлы для создания:**
- `ui/wizard/presentation/presenter.go`
- `ui/wizard/presentation/source_presenter.go`
- `ui/wizard/presentation/rules_presenter.go`
- `ui/wizard/presentation/preview_presenter.go`

**Задачи:**
1. Создать `WizardPresenter`:
   ```go
   type WizardPresenter struct {
       model    *WizardModel
       guiState *GUIState
       controller *core.AppController
   }
   
   func (p *WizardPresenter) LoadConfig() error
   func (p *WizardPresenter) OnSourceURLChanged(text string)
   func (p *WizardPresenter) OnParseClicked()
   func (p *WizardPresenter) OnSaveClicked() error
   func (p *WizardPresenter) UpdateGUI()
   ```

2. Реализовать методы синхронизации модели и GUI:
   ```go
   func (p *WizardPresenter) SyncModelToGUI() {
       // Обновить виджеты из модели
       p.guiState.SourceURLEntry.SetText(p.model.SourceURLs)
       p.guiState.ParserConfigEntry.SetText(p.model.ParserConfigJSON)
       // ...
   }
   
   func (p *WizardPresenter) SyncGUIToModel() {
       // Обновить модель из виджетов
       p.model.SourceURLs = p.guiState.SourceURLEntry.Text
       p.model.ParserConfigJSON = p.guiState.ParserConfigEntry.Text
       // ...
   }
   ```

3. Создать адаптер для бизнес-логики:
   ```go
   type BusinessLogicAdapter struct {
       presenter *WizardPresenter
   }
   
   func (a *BusinessLogicAdapter) UpdateURLStatus(status string) {
       a.presenter.guiState.URLStatusLabel.SetText(status)
   }
   ```

### Этап 4: Рефакторинг табов

**Файлы для изменения:**
- `ui/wizard/tabs/source_tab.go`
- `ui/wizard/tabs/rules_tab.go`
- `ui/wizard/tabs/preview_tab.go`

**Задачи:**
1. Изменить функции создания табов, чтобы они принимали презентер:
   ```go
   // Было:
   func CreateSourceTab(state *WizardState) fyne.CanvasObject
   
   // Станет:
   func CreateSourceTab(presenter *WizardPresenter) fyne.CanvasObject
   ```

2. Заменить прямые обращения к `state` на вызовы методов презентера:
   ```go
   // Было:
   state.SourceURLEntry.OnChanged = func(value string) {
       state.PreviewNeedsParse = true
       wizardbusiness.ApplyURLToParserConfig(state, strings.TrimSpace(value))
   }
   
   // Станет:
   sourceURLEntry.OnChanged = func(value string) {
       presenter.OnSourceURLChanged(strings.TrimSpace(value))
   }
   ```

### Этап 5: Рефакторинг wizard.go

**Файлы для изменения:**
- `ui/wizard/wizard.go`

**Задачи:**
1. Создать презентер вместо прямого использования `WizardState`:
   ```go
   func ShowConfigWizard(parent fyne.Window, controller *core.AppController) {
       model := models.NewWizardModel()
       guiState := presentation.NewGUIState(controller.UIService.Application)
       presenter := presentation.NewWizardPresenter(model, guiState, controller)
       
       // Инициализация
       if err := presenter.Initialize(); err != nil {
           // обработка ошибки
           return
       }
       
       // Создание табов через презентер
       tab1 := wizardtabs.CreateSourceTab(presenter)
       // ...
   }
   ```

### Этап 6: Написание тестов

**Файлы для создания:**
- `ui/wizard/business/generator_test.go` (обновить)
- `ui/wizard/business/parser_test.go` (обновить)
- `ui/wizard/models/wizard_model_test.go` (новый)
- `ui/wizard/presentation/presenter_test.go` (новый, с моками GUI)

**Задачи:**
1. Переписать существующие тесты для работы с `WizardModel`:
   ```go
   func TestBuildTemplateConfig(t *testing.T) {
       model := &WizardModel{
           ParserConfigJSON: `{"ParserConfig": {...}}`,
           TemplateData: templateData,
           // ...
       }
       
       result, err := BuildTemplateConfig(model, false)
       // assertions
   }
   ```

2. Создать моки для GUI-виджетов для тестирования презентеров:
   ```go
   type MockUIUpdater struct {
       StatusUpdates []string
   }
   
   func (m *MockUIUpdater) UpdateURLStatus(status string) {
       m.StatusUpdates = append(m.StatusUpdates, status)
   }
   ```

3. Добавить интеграционные тесты для презентеров

### Этап 7: Удаление старого кода и очистка

**После успешного рефакторинга - полная очистка:**
1. Удалить старые функции бизнес-логики, которые принимали `*WizardState`
2. Удалить старые методы из `WizardState`, которые перемещены в модели
3. Удалить неиспользуемые поля из `WizardState`
4. Опционально: полностью удалить `WizardState`, если он больше не используется
5. Удалить устаревшие тесты, которые тестируют старую архитектуру
6. Удалить все артефакты из прошлого:
   - Временные решения и костыли
   - Комментарии о "TODO: refactor", "temporary solution"
   - Мертвый код, неиспользуемые функции
   - Дубли кода, если они обнаружены
7. Провести финальную проверку кода на чистоту и управляемость

## Порядок выполнения (последовательность)

### Фаза 1: Подготовка
1. Создать `ui/wizard/models/` с `WizardModel` и `RuleState`
2. Создать `GUIState` структуру (в `ui/wizard/presentation/` или отдельном файле)
3. Добавить тесты для моделей

### Фаза 2: Рефакторинг бизнес-логики
5. Создать новые функции бизнес-логики, принимающие `*WizardModel`
6. Создать интерфейсы `UIUpdater` для обновления GUI
7. Переписать существующие функции бизнес-логики на новую архитектуру (писать заново, чисто, с учетом опыта)
8. Удалить старые функции бизнес-логики (которые принимали `*WizardState`)
9. Написать новые тесты для новой бизнес-логики (удалить старые тесты, которые тестируют устаревшую архитектуру)

### Фаза 3: Создание Presentation Layer
10. Создать `WizardPresenter`
11. Создать презентеры для каждого таба
12. Реализовать синхронизацию модель ↔ GUI
13. Добавить тесты для презентеров (с моками)

### Фаза 4: Рефакторинг UI
14. Изменить `wizard.go` для использования презентера (полностью переписать)
15. Рефакторить табы один за другим (переписать полностью):
    - Source tab
    - Rules tab  
    - Preview tab
16. Тестировать каждый таб после рефакторинга (проверить поведение GUI)

### Фаза 5: Завершение и очистка
17. Удалить старый код из `WizardState` (неиспользуемые поля и методы)
18. Опционально: полностью удалить `WizardState`, если он больше не используется
19. Удалить все артефакты из прошлого: временные решения, комментарии о "TODO: refactor", мертвый код
20. Удалить устаревшие тесты, которые тестируют старую архитектуру
21. Провести финальную проверку на дублирование кода и удалить дубли
22. Обновить документацию
23. Финальное тестирование (проверить поведение GUI и функциональность)

## Преимущества после рефакторинга

1. **Тестируемость**
   - Бизнес-логика тестируется без Fyne
   - Модели данных тестируются изолированно
   - Презентеры тестируются с моками GUI

2. **Разделение ответственности**
   - Модели: только данные
   - Бизнес-логика: только алгоритмы
   - Презентеры: связь между GUI и бизнес-логикой
   - GUI: только отображение

3. **Гибкость**
   - Легко заменить GUI-фреймворк (достаточно переписать Presentation Layer)
   - Легко добавить новые представления (CLI, API, etc.)
   - Бизнес-логика переиспользуется

4. **Поддерживаемость**
   - Четкая структура кода
   - Легче найти и исправить баги
   - Легче добавлять новые функции

## Риски и митигация

### Риск 1: Большой объем изменений
**Митигация:** 
- Постепенный рефакторинг по этапам (модели → бизнес-логика → презентеры → UI)
- Тщательное тестирование поведения GUI после каждого этапа
- Не нужно поддерживать старый API - можно переписывать полностью

### Риск 2: Регрессии в функциональности
**Митигация:** 
- Обширное тестирование на каждом этапе
- Интеграционные тесты перед удалением старого кода
- Ручное тестирование GUI после каждого этапа

### Риск 3: Производительность
**Митигация:**
- Профилирование перед и после рефакторинга
- Минимизация копирования данных (использование указателей)
- Оптимизация синхронизации модель ↔ GUI

## Метрики успеха

1. **Покрытие тестами:**
   - Бизнес-логика: >80%
   - Модели: >90%
   - Презентеры: >70%

2. **Зависимости:**
   - Бизнес-логика: 0 зависимостей от Fyne
   - Модели: 0 зависимостей от Fyne
   - Презентеры: только Presentation Layer зависит от Fyne

3. **Функциональность:**
   - Все существующие функции работают
   - Нет регрессий в GUI
   - Производительность не ухудшилась

## Анализ дублирования с core/config/parser

✅ **Дублирования в логике парсинга НЕТ**. Визард правильно использует `core/config/parser` для извлечения @ParserConfig блоков.

- `core/config/parser` - низкоуровневый парсер (извлечение из файлов, миграции версий)
- `ui/wizard/business/parser.go` - высокоуровневая бизнес-логика визарда (обработка URL, применение к конфигурации, preview)

Визард использует `parser.ExtractParserConfig()` из `core/config/parser`, не дублируя функциональность.

### ⚠️ Дублирование констант и валидации

**Проблема:** Есть дублирование констант и валидации размера файлов:

1. **Константы (одинаковые значения):**
   - `core/config/parser/factory.go`: `MaxConfigFileSize = 50 * 1024 * 1024` (50 MB)
   - `ui/wizard/utils/constants.go`: `MaxJSONConfigSize = 50 * 1024 * 1024` (50 MB)

2. **Валидация размера файла:**
   - В `core/config/parser/factory.go` встроена в `ExtractParserConfig()` (проверка размера файла и данных)
   - В `ui/wizard/business/loader.go` есть повторная проверка размера файла перед вызовом `parser.ExtractParserConfig()`
   - В `ui/wizard/business/validator.go` есть функция `ValidateJSONSize()` которая дублирует проверку

**Решение при рефакторинге:**

1. Удалить константу `MaxJSONConfigSize` из `ui/wizard/utils/constants.go`
2. Использовать `parser.MaxConfigFileSize` из `core/config/parser` во всех местах
3. Удалить дублирующую проверку размера файла из `loader.go` (это уже делает `ExtractParserConfig()`)
4. Обновить `ValidateJSONSize()` для использования константы из core (или передавать лимит как параметр)

**Задачи для рефакторинга:**
- [ ] Заменить все использования `wizardutils.MaxJSONConfigSize` на `parser.MaxConfigFileSize`
- [ ] Удалить дублирующую проверку размера файла из `loader.go`
- [ ] Обновить `ValidateJSONSize()` для использования константы из core
- [ ] Удалить константу `MaxJSONConfigSize` из `wizard/utils/constants.go`

## Дополнительные рекомендации

1. **Использовать интерфейсы для абстракции:**
   ```go
   type ConfigLoader interface {
       LoadConfig(path string) (*WizardModel, error)
   }
   
   type ConfigSaver interface {
       SaveConfig(model *WizardModel, path string) error
   }
   ```

2. **Event-driven подход для обновлений:**
   ```go
   type ModelChangeEvent struct {
       Field string
       Value interface{}
   }
   
   func (p *WizardPresenter) OnModelChanged(event ModelChangeEvent) {
       p.SyncModelToGUI()
   }
   ```

3. **Валидация в моделях:**
   ```go
   func (m *WizardModel) Validate() error {
       if m.ParserConfigJSON == "" {
           return fmt.Errorf("ParserConfig is required")
       }
       // ...
   }
   ```

4. **Использовать builder pattern для сложных операций:**
   ```go
   builder := NewConfigBuilder(model)
   builder.WithTemplate(templateData)
   builder.WithRules(rules)
   config, err := builder.Build()
   ```

## Заключение

Данный рефакторинг позволит:
- Разделить GUI и бизнес-логику
- Сделать код тестируемым
- Улучшить архитектуру и поддерживаемость
- Сохранить функциональность и производительность

Рефакторинг должен выполняться постепенно с тщательным тестированием на каждом этапе.

## Принятые решения по архитектурным вопросам

Все открытые вопросы уточнены и решены:

### 1. Расположение компонентов

- **GUIState:** `ui/wizard/presentation/gui_state.go` (отдельный файл)
- **UIUpdater интерфейс:** `ui/wizard/business/ui_updater.go` (в business, так как используется бизнес-логикой)
- **helpers.go:** 
  - Логирование: убрать обертки (DebugLog, InfoLog, ErrorLog), использовать `internal/debuglog` напрямую
  - `SafeFyneDo`: переместить в `ui/wizard/presentation/utils.go` (так как связан с GUI)

### 2. Методы WizardState

**Разделение по ответственности:**

- **В бизнес-логику (`business/`):**
  - `SaveConfigWithBackup()` → `business/saver.go`
  - `GetAvailableOutbounds()` → `business/outbound.go`
  - `EnsureFinalSelected()` → `business/outbound.go`

- **В презентер (`presentation/`):**
  - `SetCheckURLState()` → методы презентера для управления UI
  - `SetSaveState()` → методы презентера для управления UI
  - `SetTemplatePreviewText()` → методы презентера для обновления preview
  - `RefreshOutboundOptions()` → методы презентера для обновления виджетов
  - `InitializeTemplateState()` → инициализация в презентере

### 3. Связь данных и GUI

- **RuleState ↔ виджеты:**
  - Структура-обертка `RuleWidget` в `gui_state.go`:
    ```go
    type RuleWidget struct {
        Select *widget.Select
        RuleState *RuleState  // ссылка на правило из модели
    }
    ```
  - В GUIState: `RuleOutboundSelects []*RuleWidget`

- **Флаги состояния (разделение):**
  - **В модель (`WizardModel`):** бизнес-флаги операций
    - `AutoParseInProgress`
    - `PreviewGenerationInProgress`
  - **В GUIState:** UI-флаги операций и блокировки
    - `CheckURLInProgress`
    - `SaveInProgress`
    - `ParserConfigUpdating` (для блокировки рекурсивных обновлений)
    - `UpdatingOutboundOptions` (для блокировки callbacks)

### 4. Детали реализации

- **Синхронизация модель ↔ GUI:** по требованию (явный вызов `SyncModelToGUI()` и `SyncGUIToModel()` в нужных местах)

- **Доступ к сервисам:**
  - Controller НЕ хранится в модели (модель чистая, только данные)
  - Бизнес-логика получает сервисы через параметры функций (например, `SaveConfig(fileService, model)`)
  - Controller хранится в презентере, презентер передает нужные сервисы в бизнес-логику

- **Открытые диалоги:** хранятся в презентере (`WizardPresenter.OpenRuleDialogs map[int]fyne.Window`)

- **Константы (DefaultOutboundTag, RejectActionName, RejectActionMethod):** переместить в `ui/wizard/models/constants.go`

- **TemplateData загрузка:**
  - Отдельный сервис/loader с интерфейсом `TemplateLoader`
  - Презентер получает через dependency injection (можно мокировать в тестах)
  - Загрузка в отдельном модуле, не в презентере (для тестируемости)

### 5. Тестирование

- **Unit-тесты:** для бизнес-логики (`business/`) и моделей (`models/`)
- **Презентеры:** тестирование отложено (будет добавлено позже, если потребуется)

### 6. Обработка ошибок

- Бизнес-логика возвращает ошибки (не показывает диалоги)
- Презентер обрабатывает ошибки (показывает диалоги, статусы, логирует)
- Разделение ответственности: бизнес-логика валидирует, презентер показывает

