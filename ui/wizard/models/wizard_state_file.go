// Package models содержит модели данных визарда конфигурации.
//
// Файл wizard_state_file.go определяет структуры данных для сериализации состояния визарда в JSON.
//
// WizardStateFile - основная структура для сохранения/загрузки состояния визарда:
//   - Метаданные (version, id, comment, created_at, updated_at)
//   - ParserConfig - конфигурация парсера (единственный источник @ParserConfig)
//   - ConfigParams - параметры конфигурации (route.final, experimental.clash_api.secret и т.д.)
//   - SelectableRuleStates - состояния правил из шаблона
//   - CustomRules - пользовательские правила
//
// PersistedRuleState - сериализуемая версия RuleState с дополнительным полем type
// PersistedTemplateSelectableRule - сериализуемая версия TemplateSelectableRule
// WizardStateMetadata - метаданные состояния для списка (без полного содержимого)
//
// Используется в:
//   - business/state_store.go - для сохранения/загрузки состояний
//   - presentation/presenter.go - для создания состояния из модели и восстановления модели из состояния
package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"singbox-launcher/core/config"
	wizardtemplate "singbox-launcher/ui/wizard/template"
)

const (
	// WizardStateVersion - версия формата файла состояния
	WizardStateVersion = 1

	// MaxStateIDLength - максимальная длина ID состояния
	MaxStateIDLength = 50

	// StateFileName - имя файла текущего состояния
	StateFileName = "state.json"
)

var (
	// stateIDRegex - регулярное выражение для валидации ID состояния
	// Разрешены только: буквы (a-z, A-Z), цифры (0-9), дефис (-), подчёркивание (_)
	stateIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// WizardStateFile представляет сериализуемое состояние визарда.
type WizardStateFile struct {
	Version              int                    `json:"version"`
	ID                   string                 `json:"id,omitempty"` // Опционально для state.json, обязательно для именованных состояний
	Comment              string                 `json:"comment,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	ParserConfig         config.ParserConfig    `json:"parser_config"`
	ConfigParams         []ConfigParam          `json:"config_params"`
	SelectableRuleStates []PersistedRuleState   `json:"selectable_rule_states"`
	CustomRules          []PersistedRuleState   `json:"custom_rules"`
}

// ConfigParam представляет параметр конфигурации.
type ConfigParam struct {
	Name  string `json:"name"`  // Путь к параметру в точечной нотации (например, "route.final")
	Value string `json:"value"` // Значение параметра
}

// PersistedRuleState - сериализуемая версия RuleState с дополнительным полем type.
type PersistedRuleState struct {
	Type             string                          `json:"type"` // "System" для системных правил, или редактируемый тип для пользовательских
	Rule             PersistedTemplateSelectableRule `json:"rule"`
	Enabled          bool                            `json:"enabled"`
	SelectedOutbound string                          `json:"selected_outbound"`
}

// PersistedTemplateSelectableRule - сериализуемая версия TemplateSelectableRule.
type PersistedTemplateSelectableRule struct {
	Label           string                 `json:"label"`
	Description     string                 `json:"description"`
	Raw             map[string]interface{} `json:"raw"`
	DefaultOutbound string                 `json:"default_outbound"`
	HasOutbound     bool                   `json:"has_outbound"`
	IsDefault       bool                   `json:"is_default"`
}

// WizardStateMetadata - метаданные состояния для списка (без полного содержимого).
type WizardStateMetadata struct {
	ID        string    `json:"id"`
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsCurrent bool      `json:"is_current"` // true если это state.json
}

// ValidateStateID проверяет валидность ID состояния.
// Разрешены только: буквы (a-z, A-Z), цифры (0-9), дефис (-), подчёркивание (_)
// Максимальная длина: 50 символов
func ValidateStateID(id string) error {
	if id == "" {
		return fmt.Errorf("state ID cannot be empty")
	}
	if len(id) > MaxStateIDLength {
		return fmt.Errorf("state ID exceeds maximum length of %d characters", MaxStateIDLength)
	}
	if !stateIDRegex.MatchString(id) {
		return fmt.Errorf("state ID can only contain letters (a-z, A-Z), numbers (0-9), hyphen (-), and underscore (_)")
	}
	return nil
}

// ToPersistedRuleState преобразует RuleState в PersistedRuleState.
// Определяет тип правила на основе rule.raw, если type не задан явно.
func ToPersistedRuleState(ruleState *RuleState, ruleType string) PersistedRuleState {
	// Если тип не задан, определяем его из rule.raw
	if ruleType == "" {
		ruleType = determineRuleType(ruleState.Rule.Raw)
	}

	return PersistedRuleState{
		Type:    ruleType,
		Rule:    ToPersistedTemplateSelectableRule(ruleState.Rule),
		Enabled: ruleState.Enabled,
		SelectedOutbound: ruleState.SelectedOutbound,
	}
}

// ToPersistedTemplateSelectableRule преобразует TemplateSelectableRule в PersistedTemplateSelectableRule.
func ToPersistedTemplateSelectableRule(rule wizardtemplate.TemplateSelectableRule) PersistedTemplateSelectableRule {
	return PersistedTemplateSelectableRule{
		Label:           rule.Label,
		Description:     rule.Description,
		Raw:             rule.Raw,
		DefaultOutbound: rule.DefaultOutbound,
		HasOutbound:     rule.HasOutbound,
		IsDefault:       rule.IsDefault,
	}
}

// ToRuleState преобразует PersistedRuleState в RuleState.
func (prs *PersistedRuleState) ToRuleState() *RuleState {
	return &RuleState{
		Rule: wizardtemplate.TemplateSelectableRule{
			Label:           prs.Rule.Label,
			Description:     prs.Rule.Description,
			Raw:             prs.Rule.Raw,
			DefaultOutbound: prs.Rule.DefaultOutbound,
			HasOutbound:     prs.Rule.HasOutbound,
			IsDefault:       prs.Rule.IsDefault,
		},
		Enabled:          prs.Enabled,
		SelectedOutbound: prs.SelectedOutbound,
	}
}

// determineRuleType определяет тип правила на основе rule.raw.
// Используется для системных правил, если type не задан явно.
func determineRuleType(raw map[string]interface{}) string {
	if raw == nil {
		return "Custom JSON"
	}

	// Проверяем наличие полей для определения типа
	if _, ok := raw["ip_cidr"]; ok {
		return "IP Addresses (CIDR)"
	}
	if _, ok := raw["domain_regex"]; ok {
		return "Domains/URLs"
	}
	if _, ok := raw["domain"]; ok {
		return "Domains/URLs"
	}
	if _, ok := raw["domain_suffix"]; ok {
		return "Domains/URLs"
	}
	if _, ok := raw["domain_keyword"]; ok {
		return "Domains/URLs"
	}
	if _, ok := raw["process_name"]; ok {
		return "Processes"
	}

	// По умолчанию для системных правил
	return "System"
}

// MarshalJSON кастомная сериализация для правильного формата времени.
func (wsf *WizardStateFile) MarshalJSON() ([]byte, error) {
	type Alias WizardStateFile
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Alias:     (*Alias)(wsf),
		CreatedAt: wsf.CreatedAt.Format(time.RFC3339),
		UpdatedAt: wsf.UpdatedAt.Format(time.RFC3339),
	})
}

// UnmarshalJSON кастомная десериализация для правильного формата времени.
func (wsf *WizardStateFile) UnmarshalJSON(data []byte) error {
	type Alias WizardStateFile
	aux := &struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Alias: (*Alias)(wsf),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Парсим время
	if aux.CreatedAt != "" {
		createdAt, err := time.Parse(time.RFC3339, aux.CreatedAt)
		if err != nil {
			return fmt.Errorf("invalid created_at format: %w", err)
		}
		wsf.CreatedAt = createdAt
	}
	if aux.UpdatedAt != "" {
		updatedAt, err := time.Parse(time.RFC3339, aux.UpdatedAt)
		if err != nil {
			return fmt.Errorf("invalid updated_at format: %w", err)
		}
		wsf.UpdatedAt = updatedAt
	}

	return nil
}

// MarshalJSON кастомная сериализация для WizardStateMetadata.
func (wsm *WizardStateMetadata) MarshalJSON() ([]byte, error) {
	type Alias WizardStateMetadata
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Alias:     (*Alias)(wsm),
		CreatedAt: wsm.CreatedAt.Format(time.RFC3339),
		UpdatedAt: wsm.UpdatedAt.Format(time.RFC3339),
	})
}

// UnmarshalJSON кастомная десериализация для WizardStateMetadata.
func (wsm *WizardStateMetadata) UnmarshalJSON(data []byte) error {
	type Alias WizardStateMetadata
	aux := &struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Alias: (*Alias)(wsm),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Парсим время
	if aux.CreatedAt != "" {
		createdAt, err := time.Parse(time.RFC3339, aux.CreatedAt)
		if err != nil {
			return fmt.Errorf("invalid created_at format: %w", err)
		}
		wsm.CreatedAt = createdAt
	}
	if aux.UpdatedAt != "" {
		updatedAt, err := time.Parse(time.RFC3339, aux.UpdatedAt)
		if err != nil {
			return fmt.Errorf("invalid updated_at format: %w", err)
		}
		wsm.UpdatedAt = updatedAt
	}

	return nil
}

