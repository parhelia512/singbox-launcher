// Package tabs содержит UI компоненты для табов визарда конфигурации.
//
// Файл rules_tab.go содержит функцию CreateRulesTab, которая создает UI второго таба визарда:
//   - Отображение правил маршрутизации из шаблона (SelectableRuleStates)
//   - Выбор outbound для каждого правила через Select виджеты
//   - Отображение пользовательских правил (CustomRules)
//   - Кнопки добавления, редактирования и удаления правил
//   - Выбор финального outbound (FinalOutboundSelect)
//
// Каждый таб визарда имеет свою отдельную ответственность и логику UI.
// Содержит сложную логику управления виджетами правил (RuleWidget) и их синхронизации с моделью.
//
// Используется в:
//   - wizard.go - при создании окна визарда, вызывается CreateRulesTab(presenter, showAddRuleDialog)
//   - presenter_rules.go - RefreshRulesTab вызывает CreateRulesTab для обновления содержимого таба
//
// Взаимодействует с:
//   - presenter - все действия пользователя обрабатываются через методы presenter
//   - dialogs/add_rule_dialog.go - вызывает ShowAddRuleDialog для добавления/редактирования правил
package tabs

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/internal/debuglog"
	wizardbusiness "singbox-launcher/ui/wizard/business"
	wizardmodels "singbox-launcher/ui/wizard/models"
	wizardpresentation "singbox-launcher/ui/wizard/presentation"
	wizardtemplate "singbox-launcher/ui/wizard/template"
)

// ShowAddRuleDialogFunc is a function type for showing the add rule dialog.
type ShowAddRuleDialogFunc func(p *wizardpresentation.WizardPresenter, editRule *wizardmodels.RuleState, ruleIndex int)

// CreateRulesTab creates the Rules tab UI.
// showAddRuleDialog is a function that will be called to show the add rule dialog.
func CreateRulesTab(presenter *wizardpresentation.WizardPresenter, showAddRuleDialog ShowAddRuleDialogFunc) fyne.CanvasObject {
	model := presenter.Model()
	guiState := presenter.GUIState()

	// Validate template data
	if model.TemplateData == nil {
		return createTemplateNotFoundMessage()
	}

	// Initialize state
	initializeRulesTabState(presenter, model, guiState)
	availableOutbounds := wizardbusiness.EnsureDefaultAvailableOutbounds(wizardbusiness.GetAvailableOutbounds(model))

	// Create UI components
	rulesBox := createSelectableRulesUI(presenter, model, guiState, availableOutbounds)
	// Type assertion для добавления элементов
	if vbox, ok := rulesBox.(interface{ Add(...fyne.CanvasObject) }); ok {
		createCustomRulesUI(presenter, model, guiState, availableOutbounds, showAddRuleDialog, vbox)
		createAddRuleButton(presenter, showAddRuleDialog, vbox)
	}
	finalSelect := createFinalOutboundSelect(presenter, model, guiState, availableOutbounds)

	// Create scrollable container
	rulesScroll := CreateRulesScroll(guiState, rulesBox)

	// RefreshOutboundOptions will reset UpdatingOutboundOptions flag and hasChanges after all SetSelected() calls
	presenter.RefreshOutboundOptions()

	// Build final container
	return buildRulesTabContainer(rulesScroll, finalSelect)
}

// createTemplateNotFoundMessage создает сообщение об отсутствии шаблона.
func createTemplateNotFoundMessage() fyne.CanvasObject {
	templateFileName := wizardtemplate.GetTemplateFileName()
	return container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Template file bin/%s not found.", templateFileName)),
		widget.NewLabel("Create the template file to enable this tab."),
	)
}

// initializeRulesTabState инициализирует состояние таба правил.
func initializeRulesTabState(presenter *wizardpresentation.WizardPresenter, model *wizardmodels.WizardModel, guiState *wizardpresentation.GUIState) {
	presenter.InitializeTemplateState()

	// Set flag to block callbacks during initialization
	guiState.UpdatingOutboundOptions = true
	debuglog.DebugLog("rules_tab: UpdatingOutboundOptions set to true before creating widgets")

	// Initialize CustomRules if needed
	if model.CustomRules == nil {
		model.CustomRules = make([]*wizardmodels.RuleState, 0)
	}
}

// createSelectableRulesUI создает UI для selectable rules из шаблона.
// Возвращает VBox контейнер для добавления элементов.
func createSelectableRulesUI(presenter *wizardpresentation.WizardPresenter, model *wizardmodels.WizardModel, guiState *wizardpresentation.GUIState, availableOutbounds []string) fyne.CanvasObject {
	rulesBox := container.NewVBox()

	if len(model.SelectableRuleStates) == 0 {
		rulesBox.Add(widget.NewLabel("No selectable rules defined in template."))
		return rulesBox
	}

	for i := range model.SelectableRuleStates {
		ruleState := model.SelectableRuleStates[i]
		idx := i

		// Create outbound selector if rule has outbound field
		outboundSelect, outboundRow := createOutboundSelectorForSelectableRule(
			presenter, model, guiState, ruleState, idx, availableOutbounds,
		)

		// Create RuleWidget and add to GUIState
		ruleWidget := &wizardpresentation.RuleWidget{
			Select:    outboundSelect,
			RuleState: ruleState,
		}
		guiState.RuleOutboundSelects = append(guiState.RuleOutboundSelects, ruleWidget)

		// Create checkbox with callback
		checkbox := createSelectableRuleCheckbox(presenter, model, guiState, ruleState, idx, outboundSelect)

		// Create row content
		rowContent := createSelectableRuleRowContent(ruleState, guiState, checkbox, outboundRow)
		rulesBox.Add(container.NewHBox(rowContent...))
	}

	return rulesBox
}

// createOutboundSelectorForSelectableRule создает селектор outbound для selectable rule.
func createOutboundSelectorForSelectableRule(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	ruleState *wizardmodels.RuleState,
	idx int,
	availableOutbounds []string,
) (*widget.Select, fyne.CanvasObject) {
	if !ruleState.Rule.HasOutbound {
		return nil, nil
	}

	wizardmodels.EnsureDefaultOutbound(ruleState, availableOutbounds)
	outboundSelect := widget.NewSelect(availableOutbounds, func(value string) {
		// Ignore callback during programmatic update
		if guiState.UpdatingOutboundOptions {
			return
		}
		model.SelectableRuleStates[idx].SelectedOutbound = value
		model.TemplatePreviewNeedsUpdate = true
		presenter.MarkAsChanged()
	})
	outboundSelect.SetSelected(ruleState.SelectedOutbound)
	if !ruleState.Enabled {
		outboundSelect.Disable()
	}

	outboundRow := container.NewHBox(
		widget.NewLabel("Outbound:"),
		outboundSelect,
	)

	return outboundSelect, outboundRow
}

// createSelectableRuleCheckbox создает checkbox для selectable rule.
func createSelectableRuleCheckbox(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	ruleState *wizardmodels.RuleState,
	idx int,
	outboundSelect *widget.Select,
) *widget.Check {
	checkbox := widget.NewCheck(ruleState.Rule.Label, func(val bool) {
		// Always update model and UI state to keep them in sync
		model.SelectableRuleStates[idx].Enabled = val
		model.TemplatePreviewNeedsUpdate = true

		if outboundSelect != nil {
			if val {
				outboundSelect.Enable()
			} else {
				outboundSelect.Disable()
			}
		}

		// Only mark as changed if not during programmatic update
		if !guiState.UpdatingOutboundOptions {
			presenter.MarkAsChanged()
		}
	})
	checkbox.SetChecked(ruleState.Enabled)
	return checkbox
}

// createSelectableRuleRowContent создает содержимое строки для selectable rule.
func createSelectableRuleRowContent(
	ruleState *wizardmodels.RuleState,
	guiState *wizardpresentation.GUIState,
	checkbox *widget.Check,
	outboundRow fyne.CanvasObject,
) []fyne.CanvasObject {
	// Create checkbox container with optional info button for description
	checkboxContainer := container.NewHBox(checkbox)
	if ruleState.Rule.Description != "" {
		infoButton := widget.NewButton("?", func() {
			dialog.ShowInformation(ruleState.Rule.Label, ruleState.Rule.Description, guiState.Window)
		})
		infoButton.Importance = widget.LowImportance
		checkboxContainer.Add(infoButton)
	}

	rowContent := []fyne.CanvasObject{checkboxContainer, layout.NewSpacer()}
	if outboundRow != nil {
		rowContent = append(rowContent, outboundRow)
	}

	return rowContent
}

// rulesBoxAdder интерфейс для добавления элементов в контейнер.
type rulesBoxAdder interface {
	Add(...fyne.CanvasObject)
}

// createCustomRulesUI создает UI для пользовательских правил.
func createCustomRulesUI(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	availableOutbounds []string,
	showAddRuleDialog ShowAddRuleDialogFunc,
	rulesBox rulesBoxAdder,
) {
	for i := range model.CustomRules {
		customRule := model.CustomRules[i]
		idx := i

		// Create outbound selector
		outboundSelect := createOutboundSelectorForCustomRule(
			presenter, model, guiState, customRule, idx, availableOutbounds,
		)

		// Create RuleWidget for custom rule
		customRuleWidget := &wizardpresentation.RuleWidget{
			Select:    outboundSelect,
			RuleState: customRule,
		}
		guiState.RuleOutboundSelects = append(guiState.RuleOutboundSelects, customRuleWidget)

		// Create action buttons
		editButton, deleteButton := createCustomRuleActionButtons(
			presenter, model, guiState, customRule, idx, showAddRuleDialog,
		)

		// Create checkbox
		checkbox := createCustomRuleCheckbox(presenter, model, guiState, customRule, idx, outboundSelect)

		// Create row content
		rowContent := createCustomRuleRowContent(checkbox, editButton, deleteButton, outboundSelect)
		rulesBox.Add(container.NewHBox(rowContent...))
	}
}

// createOutboundSelectorForCustomRule создает селектор outbound для custom rule.
func createOutboundSelectorForCustomRule(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	customRule *wizardmodels.RuleState,
	idx int,
	availableOutbounds []string,
) *widget.Select {
	wizardmodels.EnsureDefaultOutbound(customRule, availableOutbounds)

	outboundSelect := widget.NewSelect(availableOutbounds, func(value string) {
		if guiState.UpdatingOutboundOptions {
			return
		}
		model.CustomRules[idx].SelectedOutbound = value
		model.TemplatePreviewNeedsUpdate = true
		presenter.MarkAsChanged()
	})
	outboundSelect.SetSelected(customRule.SelectedOutbound)
	if !customRule.Enabled {
		outboundSelect.Disable()
	}

	return outboundSelect
}

// createCustomRuleActionButtons создает кнопки редактирования и удаления для custom rule.
func createCustomRuleActionButtons(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	customRule *wizardmodels.RuleState,
	idx int,
	showAddRuleDialog ShowAddRuleDialogFunc,
) (*widget.Button, *widget.Button) {
	// Edit button
	editButton := widget.NewButton("✏️", func() {
		showAddRuleDialog(presenter, customRule, idx)
	})
	editButton.Importance = widget.LowImportance

	// Delete button
	deleteButton := widget.NewButton("❌", func() {
		deleteCustomRule(presenter, model, guiState, customRule, showAddRuleDialog)
	})
	deleteButton.Importance = widget.LowImportance

	return editButton, deleteButton
}

// deleteCustomRule удаляет пользовательское правило.
func deleteCustomRule(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	customRule *wizardmodels.RuleState,
	showAddRuleDialog ShowAddRuleDialogFunc,
) {
	// Find and remove rule from model
	for i, rule := range model.CustomRules {
		if rule == customRule {
			model.CustomRules = append(model.CustomRules[:i], model.CustomRules[i+1:]...)
			break
		}
	}

	// Remove from GUIState
	newRuleWidgets := make([]*wizardpresentation.RuleWidget, 0, len(guiState.RuleOutboundSelects)-1)
	for _, rw := range guiState.RuleOutboundSelects {
		if r, ok := rw.RuleState.(*wizardmodels.RuleState); ok && r != customRule {
			newRuleWidgets = append(newRuleWidgets, rw)
		}
	}
	guiState.RuleOutboundSelects = newRuleWidgets

	model.TemplatePreviewNeedsUpdate = true
	presenter.MarkAsChanged()

	// Recreate tab content
	refreshWrapper := func(p *wizardpresentation.WizardPresenter) fyne.CanvasObject {
		return CreateRulesTab(p, showAddRuleDialog)
	}
	presenter.RefreshRulesTab(refreshWrapper)
}

// createCustomRuleCheckbox создает checkbox для custom rule.
func createCustomRuleCheckbox(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	customRule *wizardmodels.RuleState,
	idx int,
	outboundSelect *widget.Select,
) *widget.Check {
	checkbox := widget.NewCheck(customRule.Rule.Label, func(val bool) {
		// Always update model and UI state to keep them in sync
		model.CustomRules[idx].Enabled = val
		model.TemplatePreviewNeedsUpdate = true

		if val {
			outboundSelect.Enable()
		} else {
			outboundSelect.Disable()
		}

		// Only mark as changed if not during programmatic update
		if !guiState.UpdatingOutboundOptions {
			presenter.MarkAsChanged()
		}
	})
	checkbox.SetChecked(customRule.Enabled)
	return checkbox
}

// createCustomRuleRowContent создает содержимое строки для custom rule.
func createCustomRuleRowContent(
	checkbox *widget.Check,
	editButton *widget.Button,
	deleteButton *widget.Button,
	outboundSelect *widget.Select,
) []fyne.CanvasObject {
	return []fyne.CanvasObject{
		checkbox,
		editButton,
		deleteButton,
		layout.NewSpacer(),
		container.NewHBox(
			widget.NewLabel("Outbound:"),
			outboundSelect,
		),
	}
}

// createAddRuleButton создает кнопку добавления правила.
func createAddRuleButton(
	presenter *wizardpresentation.WizardPresenter,
	showAddRuleDialog ShowAddRuleDialogFunc,
	rulesBox rulesBoxAdder,
) {
	addRuleButton := widget.NewButton("➕ Add Rule", func() {
		showAddRuleDialog(presenter, nil, -1)
	})
	addRuleButton.Importance = widget.LowImportance
	rulesBox.Add(addRuleButton)
}

// createFinalOutboundSelect создает селектор финального outbound.
func createFinalOutboundSelect(
	presenter *wizardpresentation.WizardPresenter,
	model *wizardmodels.WizardModel,
	guiState *wizardpresentation.GUIState,
	availableOutbounds []string,
) *widget.Select {
	// Set flag BEFORE creating finalSelect to prevent callback from firing during initialization
	guiState.UpdatingOutboundOptions = true
	debuglog.DebugLog("rules_tab: UpdatingOutboundOptions set to true before creating finalSelect")

	wizardbusiness.EnsureFinalSelected(model, availableOutbounds)
	finalSelect := widget.NewSelect(availableOutbounds, func(value string) {
		// Ignore callback during programmatic update
		if guiState.UpdatingOutboundOptions {
			return
		}
		model.SelectedFinalOutbound = value
		model.TemplatePreviewNeedsUpdate = true
		presenter.MarkAsChanged()
	})
	finalSelect.SetSelected(model.SelectedFinalOutbound)
	guiState.FinalOutboundSelect = finalSelect

	return finalSelect
}

// buildRulesTabContainer создает финальный контейнер таба правил.
func buildRulesTabContainer(rulesScroll fyne.CanvasObject, finalSelect *widget.Select) fyne.CanvasObject {
	return container.NewVBox(
		widget.NewLabel("Selectable rules"),
		rulesScroll,
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewLabel("Final outbound:"),
			finalSelect,
			layout.NewSpacer(),
		),
	)
}

// CreateRulesScroll creates a scrollable container for rules content.
func CreateRulesScroll(guiState *wizardpresentation.GUIState, content fyne.CanvasObject) fyne.CanvasObject {
	maxHeight := guiState.Window.Canvas().Size().Height * 0.65
	if maxHeight <= 0 {
		maxHeight = 430
	}
	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(0, maxHeight))
	return scroll
}
