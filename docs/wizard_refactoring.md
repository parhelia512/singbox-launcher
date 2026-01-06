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

1. **Разделить модели данных и GUI-состояние**
   - Создать чистые модели данных без зависимостей от Fyne
   - Выделить GUI-состояние в отдельную структуру

2. **Сделать бизнес-логику тестируемой**
   - Бизнес-логика должна работать только с моделями данных
   - Убрать зависимости от Fyne из бизнес-логики

3. **Ввести слой представления (Presentation Layer)**
   - Создать адаптеры/презентеры для связи GUI и бизнес-логики
   - GUI обновляется через презентеры, а не напрямую

4. **Сохранение обратной совместимости**
   - Постепенный рефакторинг
   - Возможность тестирования новых компонентов параллельно со старыми

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

3. Создать функции конвертации:
   - `WizardModelToState(model *WizardModel, gui *GUIState) *WizardState` (для обратной совместимости)
   - `StateToWizardModel(state *WizardState) *WizardModel`
   - `StateToGUIState(state *WizardState) *GUIState`

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

3. Создать промежуточные функции-обертки для обратной совместимости:
   ```go
   // Обертка для старого кода
   func CheckURLLegacy(state *WizardState) {
       model := StateToWizardModel(state)
       updater := NewStateUpdater(state)
       CheckURL(model, updater)
   }
   ```

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

### Этап 7: Удаление старого кода

**После успешного рефакторинга:**
1. Удалить функции-обертки для обратной совместимости
2. Удалить старые методы из `WizardState`, которые перемещены в модели
3. Удалить неиспользуемые поля из `WizardState`
4. Опционально: полностью удалить `WizardState`, если он больше не используется

## Порядок выполнения (последовательность)

### Фаза 1: Подготовка (обратная совместимость сохраняется)
1. Создать `ui/wizard/models/` с `WizardModel` и `RuleState`
2. Создать функции конвертации `StateToWizardModel`, `WizardModelToState`
3. Создать `GUIState` структуру
4. Добавить тесты для моделей

### Фаза 2: Рефакторинг бизнес-логики (параллельно со старым кодом)
5. Создать новые функции бизнес-логики, принимающие `*WizardModel`
6. Создать интерфейсы `UIUpdater` для обновления GUI
7. Создать функции-обертки для обратной совместимости
8. Добавить тесты для новой бизнес-логики
9. Постепенно перевести существующие тесты на новую архитектуру

### Фаза 3: Создание Presentation Layer
10. Создать `WizardPresenter`
11. Создать презентеры для каждого таба
12. Реализовать синхронизацию модель ↔ GUI
13. Добавить тесты для презентеров (с моками)

### Фаза 4: Рефакторинг UI (постепенный переход)
14. Изменить `wizard.go` для использования презентера
15. Рефакторить табы один за другим:
    - Source tab
    - Rules tab  
    - Preview tab
16. Тестировать каждый таб после рефакторинга

### Фаза 5: Завершение
17. Удалить функции-обертки
18. Удалить неиспользуемый код из `WizardState`
19. Обновить документацию
20. Финальное тестирование

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
**Митигация:** Постепенный рефакторинг с сохранением обратной совместимости на каждом этапе

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

✅ **Дублирования НЕТ**. Визард правильно использует `core/config/parser` для извлечения @ParserConfig блоков.

- `core/config/parser` - низкоуровневый парсер (извлечение из файлов, миграции версий)
- `ui/wizard/business/parser.go` - высокоуровневая бизнес-логика визарда (обработка URL, применение к конфигурации, preview)

Визард использует `parser.ExtractParserConfig()` из `core/config/parser`, не дублируя функциональность.

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

