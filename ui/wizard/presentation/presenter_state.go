// Package presentation содержит слой представления визарда конфигурации.
//
// Файл presenter_state.go содержит методы для работы с сохранением и загрузкой состояний визарда:
//   - CreateStateFromModel - создает WizardStateFile из текущей модели
//   - SaveCurrentState - сохраняет текущее состояние в state.json
//   - SaveStateAs - сохраняет состояние под новым ID
//   - LoadState - загружает состояние в модель
//   - HasUnsavedChanges - проверяет наличие несохранённых изменений
//   - MarkAsChanged - устанавливает флаг изменений
//   - MarkAsSaved - сбрасывает флаг изменений
//
// Эти методы обеспечивают работу с состояниями визарда согласно спецификации:
//   - Сохранение состояния в state.json и именованные состояния
//   - Загрузка состояния из файла с восстановлением модели
//   - Отслеживание несохранённых изменений
//
// Используется в:
//   - wizard.go - при открытии визарда для проверки state.json
//   - dialogs/*.go - для сохранения/загрузки состояний через диалоги
package presentation

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"singbox-launcher/core/config"
	"singbox-launcher/internal/debuglog"
	wizardbusiness "singbox-launcher/ui/wizard/business"
	wizardmodels "singbox-launcher/ui/wizard/models"
)

// HasUnsavedChanges проверяет наличие несохранённых изменений.
// hasChanges отслеживается как поле структуры WizardPresenter.
// Устанавливается в true при изменении данных через SyncGUIToModel (если данные реально изменились).
// Сбрасывается в false при сохранении состояния или загрузке нового состояния.
func (p *WizardPresenter) HasUnsavedChanges() bool {
	return p.hasChanges
}

// MarkAsChanged устанавливает флаг изменений.
func (p *WizardPresenter) MarkAsChanged() {
	p.hasChanges = true
	debuglog.DebugLog("MarkAsChanged: hasChanges set to true")
}

// MarkAsSaved сбрасывает флаг изменений.
func (p *WizardPresenter) MarkAsSaved() {
	p.hasChanges = false
	debuglog.DebugLog("MarkAsSaved: hasChanges reset to false")
}

// CreateStateFromModel создает WizardStateFile из текущей модели.
func (p *WizardPresenter) CreateStateFromModel(comment, id string) *wizardmodels.WizardStateFile {
	// Синхронизируем GUI с моделью перед созданием состояния
	p.SyncGUIToModel()

	// Создаём состояние
	state := &wizardmodels.WizardStateFile{
		Version:   wizardmodels.WizardStateVersion,
		ID:        id,
		Comment:   comment,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Копируем ParserConfig
	if p.model.ParserConfig != nil {
		state.ParserConfig = *p.model.ParserConfig
	}

	// Извлекаем config_params из модели
	state.ConfigParams = p.extractConfigParams()

	// Преобразуем SelectableRuleStates
	state.SelectableRuleStates = make([]wizardmodels.PersistedRuleState, 0, len(p.model.SelectableRuleStates))
	for _, ruleState := range p.model.SelectableRuleStates {
		// Определяем тип правила (по умолчанию "System" для предустановленных)
		ruleType := "System"
		persisted := wizardmodels.ToPersistedRuleState(ruleState, ruleType)
		state.SelectableRuleStates = append(state.SelectableRuleStates, persisted)
	}

	// Преобразуем CustomRules
	state.CustomRules = make([]wizardmodels.PersistedRuleState, 0, len(p.model.CustomRules))
	for _, ruleState := range p.model.CustomRules {
		// Для пользовательских правил тип определяется из rule.raw
		ruleType := wizardmodels.DetermineRuleType(ruleState.Rule.Raw)
		persisted := wizardmodels.ToPersistedRuleState(ruleState, ruleType)
		state.CustomRules = append(state.CustomRules, persisted)
	}

	return state
}

// extractConfigParams извлекает параметры конфигурации из модели.
func (p *WizardPresenter) extractConfigParams() []wizardmodels.ConfigParam {
	params := make([]wizardmodels.ConfigParam, 0)

	// Добавляем route.final
	if p.model.SelectedFinalOutbound != "" {
		params = append(params, wizardmodels.ConfigParam{
			Name:  "route.final",
			Value: p.model.SelectedFinalOutbound,
		})
	} else if p.model.TemplateData != nil && p.model.TemplateData.DefaultFinal != "" {
		// Используем значение по умолчанию из шаблона
		params = append(params, wizardmodels.ConfigParam{
			Name:  "route.final",
			Value: p.model.TemplateData.DefaultFinal,
		})
	}

	// TODO: Добавить experimental.clash_api.secret, если он есть в модели
	// Пока оставляем пустым, так как он генерируется при сохранении конфига

	return params
}

// SaveCurrentState сохраняет текущее состояние в state.json.
func (p *WizardPresenter) SaveCurrentState() error {
	debuglog.InfoLog("SaveCurrentState: called")
	// Синхронизируем GUI в модель перед сохранением
	p.SyncGUIToModel()

	state := p.CreateStateFromModel("", "")
	stateStore := p.getStateStore()

	// Получаем путь к state.json для логирования
	statesDir := filepath.Join(p.controller.FileService.ExecDir, "bin", wizardbusiness.WizardStatesDir)
	statePath := filepath.Join(statesDir, wizardmodels.StateFileName)

	debuglog.InfoLog("SaveCurrentState: saving to state.json at %s", statePath)
	if err := stateStore.SaveCurrentState(state); err != nil {
		debuglog.ErrorLog("SaveCurrentState: failed to save: %v", err)
		return fmt.Errorf("failed to save current state: %w", err)
	}

	p.MarkAsSaved()
	debuglog.InfoLog("SaveCurrentState: state.json saved successfully to %s", statePath)
	return nil
}

// SaveStateAs сохраняет состояние под новым ID с комментарием.
func (p *WizardPresenter) SaveStateAs(comment, id string) error {
	// Валидация ID
	if err := wizardmodels.ValidateStateID(id); err != nil {
		return fmt.Errorf("invalid state ID: %w", err)
	}

	state := p.CreateStateFromModel(comment, id)
	stateStore := p.getStateStore()

	if err := stateStore.SaveWizardState(state, id); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	p.MarkAsSaved()
	debuglog.InfoLog("SaveStateAs: state saved successfully with ID: %s", id)
	return nil
}

// LoadState загружает состояние в модель согласно детальной последовательности восстановления.
// Выполняет 9-шаговую последовательность восстановления WizardModel согласно спецификации.
func (p *WizardPresenter) LoadState(stateFile *wizardmodels.WizardStateFile) error {
	if stateFile == nil {
		return fmt.Errorf("state file cannot be nil")
	}

	timing := debuglog.StartTiming("loadState")
	defer timing.EndWithDefer()

	// Валидация шаблона (шаг 1)
	if p.model.TemplateData == nil {
		return fmt.Errorf("template data not available")
	}

	// Восстановление parser_config (шаг 2)
	if err := p.restoreParserConfig(stateFile); err != nil {
		return err
	}

	// Извлечение SourceURLs (шаг 3)
	p.model.SourceURLs = p.extractSourceURLsFromParserConfig(&stateFile.ParserConfig)

	// Восстановление config_params (шаг 4)
	p.restoreConfigParams(stateFile.ConfigParams)

	// Инициализация TemplateSectionSelections (шаг 5)
	p.initializeTemplateSectionSelections()

	// Восстановление SelectableRuleStates (шаг 6)
	p.restoreSelectableRuleStates(stateFile.SelectableRuleStates)

	// Восстановление CustomRules (шаг 7)
	p.restoreCustomRules(stateFile.CustomRules)

	// Установка флага для парсинга (шаг 8)
	p.model.PreviewNeedsParse = true

	// Синхронизация GUI (шаг 9)
	p.SyncModelToGUI()

	// Сбрасываем флаг изменений
	p.MarkAsSaved()

	return nil
}

// restoreParserConfig восстанавливает parser_config из состояния (шаг 2).
func (p *WizardPresenter) restoreParserConfig(stateFile *wizardmodels.WizardStateFile) error {
	if stateFile.ParserConfig.ParserConfig.Proxies == nil {
		return fmt.Errorf("invalid parser_config: Proxies is nil")
	}

	p.model.ParserConfig = &stateFile.ParserConfig

	// Сериализуем parser_config в JSON строку
	parserConfigJSON, err := wizardbusiness.SerializeParserConfig(&stateFile.ParserConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize parser_config: %w", err)
	}
	p.model.ParserConfigJSON = parserConfigJSON

	return nil
}

// initializeTemplateSectionSelections инициализирует TemplateSectionSelections (шаг 5).
// Все секции устанавливаются в true.
func (p *WizardPresenter) initializeTemplateSectionSelections() {
	p.model.TemplateSectionSelections = make(map[string]bool)
	for _, sectionKey := range p.model.TemplateData.SectionOrder {
		p.model.TemplateSectionSelections[sectionKey] = true
	}
}

// restoreSelectableRuleStates восстанавливает SelectableRuleStates из состояния (шаг 6).
func (p *WizardPresenter) restoreSelectableRuleStates(persistedRules []wizardmodels.PersistedRuleState) {
	p.model.SelectableRuleStates = make([]*wizardmodels.RuleState, 0, len(persistedRules))
	for _, persistedRule := range persistedRules {
		ruleState := persistedRule.ToRuleState()
		p.model.SelectableRuleStates = append(p.model.SelectableRuleStates, ruleState)
	}
}

// restoreCustomRules восстанавливает CustomRules из состояния (шаг 7).
func (p *WizardPresenter) restoreCustomRules(persistedRules []wizardmodels.PersistedRuleState) {
	p.model.CustomRules = make([]*wizardmodels.RuleState, 0, len(persistedRules))
	for _, persistedRule := range persistedRules {
		ruleState := persistedRule.ToRuleState()
		p.model.CustomRules = append(p.model.CustomRules, ruleState)
	}
}

// extractSourceURLsFromParserConfig извлекает SourceURLs из ParserConfig.
// Объединяет Source и Connections из всех ProxySource в одну строку, разделенную переносами строк.
func (p *WizardPresenter) extractSourceURLsFromParserConfig(parserConfig *config.ParserConfig) string {
	if parserConfig == nil || len(parserConfig.ParserConfig.Proxies) == 0 {
		return ""
	}

	lines := make([]string, 0)
	for _, proxySource := range parserConfig.ParserConfig.Proxies {
		if proxySource.Source != "" {
			lines = append(lines, proxySource.Source)
		}
		lines = append(lines, proxySource.Connections...)
	}

	return strings.Join(lines, "\n")
}

// restoreConfigParams восстанавливает config_params и маппинг в модель.
func (p *WizardPresenter) restoreConfigParams(configParams []wizardmodels.ConfigParam) {
	// Ищем route.final в параметрах
	finalOutbound := p.findConfigParamValue(configParams, "route.final")

	// Используем значение из параметров, если задано, иначе fallback на шаблон
	if finalOutbound != "" {
		p.model.SelectedFinalOutbound = finalOutbound
	} else {
		p.model.SelectedFinalOutbound = p.getDefaultFinalOutbound()
	}

	// TODO: Сохранить остальные параметры для применения при генерации конфига
	// (например, experimental.clash_api.secret)
}

// findConfigParamValue ищет значение параметра по имени.
// Возвращает пустую строку, если параметр не найден.
func (p *WizardPresenter) findConfigParamValue(configParams []wizardmodels.ConfigParam, name string) string {
	for _, param := range configParams {
		if param.Name == name {
			return param.Value
		}
	}
	return ""
}

// getDefaultFinalOutbound возвращает значение по умолчанию для final outbound из шаблона.
func (p *WizardPresenter) getDefaultFinalOutbound() string {
	if p.model.TemplateData != nil && p.model.TemplateData.DefaultFinal != "" {
		return p.model.TemplateData.DefaultFinal
	}
	return ""
}

// GetStateStore создает новый StateStore для работы с состояниями.
// Публичный метод для использования в диалогах и других компонентах.
//
// Примечание: FileServiceAdapter находится в файле с build tag cgo (saver.go),
// но это не проблема, так как весь визард компилируется с cgo.
func (p *WizardPresenter) GetStateStore() *wizardbusiness.StateStore {
	// FileServiceAdapter определен в business/saver.go с build tag cgo
	// При компиляции с cgo (как в проекте) всё работает корректно
	fileServiceAdapter := &wizardbusiness.FileServiceAdapter{FileService: p.controller.FileService}
	return wizardbusiness.NewStateStore(fileServiceAdapter)
}

// getStateStore - приватный алиас для внутреннего использования в презентере.
func (p *WizardPresenter) getStateStore() *wizardbusiness.StateStore {
	return p.GetStateStore()
}
