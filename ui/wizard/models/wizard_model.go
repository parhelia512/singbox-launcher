// Package models содержит модели данных визарда конфигурации.
//
// Файл wizard_model.go определяет WizardModel — чистую модель данных визарда без GUI зависимостей.
//
// WizardModel содержит только бизнес-данные (без Fyne виджетов):
//   - ParserConfig данные (ParserConfigJSON, ParserConfig)
//   - Источники (SourceURLs)
//   - Сгенерированные outbounds (GeneratedOutbounds, OutboundStats)
//   - Template данные (TemplateData)
//   - Правила (SelectableRuleStates, CustomRules, SelectedFinalOutbound)
//   - Флаги состояния бизнес-операций (AutoParseInProgress, PreviewGenerationInProgress)
//
// GUI-состояние (виджеты Fyne, UI-флаги) находится в presentation/GUIState.
//
// Используется в:
//   - presentation/presenter.go — WizardPresenter хранит модель и синхронизирует её с GUI
//   - business/*.go — все функции бизнес-логики работают с WizardModel
package models

import (
	"singbox-launcher/core/config"
	wizardtemplate "singbox-launcher/ui/wizard/template"
)

// Константы, связанные с бизнес-логикой визарда.
const (
	// DefaultOutboundTag — тег outbound по умолчанию для правил маршрутизации.
	DefaultOutboundTag = "direct-out"
	// RejectActionName — название действия reject в правилах маршрутизации.
	RejectActionName = "reject"
	// RejectActionMethod — метод действия reject (drop).
	RejectActionMethod = "drop"
)

// OutboundStats содержит статистику по outbounds для preview.
type OutboundStats struct {
	NodesCount           int
	LocalSelectorsCount  int
	GlobalSelectorsCount int
}

// WizardModel — модель данных визарда конфигурации.
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
	TemplateData *wizardtemplate.TemplateData

	// Правила
	SelectableRuleStates  []*RuleState
	CustomRules           []*RuleState
	SelectedFinalOutbound string

	// Флаги состояния бизнес-операций
	PreviewNeedsParse           bool
	TemplatePreviewNeedsUpdate  bool
	AutoParseInProgress         bool
	PreviewGenerationInProgress bool

	// Template preview текст (кэш для оптимизации)
	TemplatePreviewText string
}

// NewWizardModel создает новую модель визарда с начальными значениями.
func NewWizardModel() *WizardModel {
	return &WizardModel{
		PreviewNeedsParse:    true,
		SelectableRuleStates: make([]*RuleState, 0),
		CustomRules:          make([]*RuleState, 0),
		GeneratedOutbounds:   make([]string, 0),
	}
}
