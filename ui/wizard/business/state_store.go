// Package business содержит бизнес-логику визарда конфигурации.
//
// Файл state_store.go реализует хранение и загрузку состояний визарда.
//
// StateStore предоставляет методы для:
//   - Сохранения состояния в файл (SaveWizardState, SaveCurrentState)
//   - Загрузки состояния из файла (LoadWizardState, LoadCurrentState)
//   - Получения списка всех состояний (ListWizardStates)
//   - Валидации ID состояния (ValidateStateID)
//
// Состояния хранятся в директории <execDir>/bin/wizard_states/:
//   - state.json - текущее рабочее состояние
//   - <id>.json - именованные сохранённые состояния
//
// Используется в:
//   - presentation/presenter.go - для сохранения/загрузки состояний через презентер
package business

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"singbox-launcher/internal/debuglog"
	wizardmodels "singbox-launcher/ui/wizard/models"
)

const (
	// WizardStatesDir - имя директории для хранения состояний визарда
	WizardStatesDir = "wizard_states"

	// MaxStateFileSize - максимальный размер файла состояния (256 KB)
	MaxStateFileSize = 256 * 1024
)

// StateStore управляет сохранением и загрузкой состояний визарда.
type StateStore struct {
	fileService FileServiceInterface
	statesDir  string
}

// NewStateStore создает новый StateStore.
func NewStateStore(fileService FileServiceInterface) *StateStore {
	statesDir := filepath.Join(fileService.ExecDir(), "bin", WizardStatesDir)
	return &StateStore{
		fileService: fileService,
		statesDir:  statesDir,
	}
}

// ensureStatesDir создает директорию состояний, если она не существует.
func (ss *StateStore) ensureStatesDir() error {
	if err := os.MkdirAll(ss.statesDir, 0755); err != nil {
		return fmt.Errorf("failed to create wizard states directory: %w", err)
	}
	return nil
}

// getStateFilePath возвращает путь к файлу состояния по ID.
func (ss *StateStore) getStateFilePath(id string) string {
	if id == "" {
		return filepath.Join(ss.statesDir, wizardmodels.StateFileName)
	}
	return filepath.Join(ss.statesDir, id+".json")
}

// SaveWizardState сохраняет состояние в файл с указанным ID.
func (ss *StateStore) SaveWizardState(state *wizardmodels.WizardStateFile, id string) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}

	// Валидация ID
	if id != "" {
		if err := wizardmodels.ValidateStateID(id); err != nil {
			return fmt.Errorf("invalid state ID: %w", err)
		}
		// Устанавливаем ID в состояние, если он не задан
		if state.ID == "" {
			state.ID = id
		} else if state.ID != id {
			return fmt.Errorf("state ID mismatch: state has %q, but requested %q", state.ID, id)
		}
	}

	// Устанавливаем версию, если не задана
	if state.Version == 0 {
		state.Version = wizardmodels.WizardStateVersion
	}

	// Устанавливаем время создания, если не задано
	if state.CreatedAt.IsZero() {
		state.CreatedAt = time.Now().UTC()
	}

	// Обновляем время изменения
	state.UpdatedAt = time.Now().UTC()

	// Создаём директорию, если не существует
	if err := ss.ensureStatesDir(); err != nil {
		return err
	}

	// Сериализуем состояние в JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Проверка размера файла
	if len(data) > MaxStateFileSize {
		debuglog.WarnLog("SaveWizardState: state file size (%d bytes) exceeds recommended maximum (%d bytes)", len(data), MaxStateFileSize)
	}

	// Определяем путь к файлу
	filePath := ss.getStateFilePath(id)

	// Сохраняем в файл
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	debuglog.InfoLog("SaveWizardState: saved state to %s", filePath)
	return nil
}

// SaveCurrentState сохраняет состояние в state.json.
func (ss *StateStore) SaveCurrentState(state *wizardmodels.WizardStateFile) error {
	return ss.SaveWizardState(state, "")
}

// LoadWizardState загружает состояние из файла по ID.
func (ss *StateStore) LoadWizardState(id string) (*wizardmodels.WizardStateFile, error) {
	if id == "" {
		return nil, fmt.Errorf("state ID cannot be empty")
	}

	filePath := ss.getStateFilePath(id)

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("state file not found: %s", filePath)
	}

	// Читаем файл
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Проверка размера файла
	if len(data) > MaxStateFileSize {
		debuglog.WarnLog("LoadWizardState: state file size (%d bytes) exceeds recommended maximum (%d bytes)", len(data), MaxStateFileSize)
	}

	// Десериализуем JSON
	var state wizardmodels.WizardStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state file: %w", err)
	}

	// Валидация версии
	if state.Version != wizardmodels.WizardStateVersion {
		return nil, fmt.Errorf("unsupported state file version: %d (expected %d)", state.Version, wizardmodels.WizardStateVersion)
	}

	// Валидация ID (если задан, должен совпадать с именем файла)
	if state.ID != "" && state.ID != id {
		debuglog.WarnLog("LoadWizardState: state ID mismatch: file has %q, but expected %q", state.ID, id)
	}

	debuglog.InfoLog("LoadWizardState: loaded state from %s", filePath)
	return &state, nil
}

// LoadCurrentState загружает состояние из state.json.
func (ss *StateStore) LoadCurrentState() (*wizardmodels.WizardStateFile, error) {
	filePath := ss.getStateFilePath("")

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("state.json not found")
	}

	// Читаем файл
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state.json: %w", err)
	}

	// Десериализуем JSON
	var state wizardmodels.WizardStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state.json: %w", err)
	}

	// Валидация версии
	if state.Version != wizardmodels.WizardStateVersion {
		return nil, fmt.Errorf("unsupported state file version: %d (expected %d)", state.Version, wizardmodels.WizardStateVersion)
	}

	debuglog.InfoLog("LoadCurrentState: loaded state from %s", filePath)
	return &state, nil
}

// ListWizardStates возвращает список всех сохранённых состояний с метаданными.
func (ss *StateStore) ListWizardStates() ([]wizardmodels.WizardStateMetadata, error) {
	// Создаём директорию, если не существует (на случай, если состояний ещё нет)
	if err := ss.ensureStatesDir(); err != nil {
		return nil, err
	}

	// Читаем все .json файлы из директории
	entries, err := os.ReadDir(ss.statesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read wizard states directory: %w", err)
	}

	var states []wizardmodels.WizardStateMetadata

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		if !strings.HasSuffix(fileName, ".json") {
			continue
		}

		// Определяем ID из имени файла
		var id string
		var isCurrent bool
		if fileName == wizardmodels.StateFileName {
			id = ""
			isCurrent = true
		} else {
			id = strings.TrimSuffix(fileName, ".json")
			isCurrent = false
		}

		// Загружаем метаданные из файла
		filePath := filepath.Join(ss.statesDir, fileName)
		data, err := os.ReadFile(filePath)
		if err != nil {
			debuglog.WarnLog("ListWizardStates: failed to read %s: %v", filePath, err)
			continue
		}

		// Парсим только метаданные (не весь файл)
		var state struct {
			ID        string    `json:"id,omitempty"`
			Comment   string    `json:"comment,omitempty"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		}

		// Используем кастомный UnmarshalJSON для времени
		var aux struct {
			ID        string `json:"id,omitempty"`
			Comment   string `json:"comment,omitempty"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		}
		if err := json.Unmarshal(data, &aux); err != nil {
			debuglog.WarnLog("ListWizardStates: failed to parse %s: %v", filePath, err)
			continue
		}

		state.ID = aux.ID
		state.Comment = aux.Comment

		// Получаем время модификации файла как fallback
		fileInfo, err := entry.Info()
		var modTime time.Time
		if err != nil {
			modTime = time.Now()
		} else {
			modTime = fileInfo.ModTime()
		}

		// Парсим время
		if aux.CreatedAt != "" {
			createdAt, err := time.Parse(time.RFC3339, aux.CreatedAt)
			if err != nil {
				debuglog.WarnLog("ListWizardStates: invalid created_at in %s: %v", filePath, err)
				createdAt = modTime
			}
			state.CreatedAt = createdAt
		} else {
			state.CreatedAt = modTime
		}

		if aux.UpdatedAt != "" {
			updatedAt, err := time.Parse(time.RFC3339, aux.UpdatedAt)
			if err != nil {
				debuglog.WarnLog("ListWizardStates: invalid updated_at in %s: %v", filePath, err)
				updatedAt = modTime
			}
			state.UpdatedAt = updatedAt
		} else {
			state.UpdatedAt = modTime
		}

		// Если ID не задан в файле, используем имя файла
		if state.ID == "" && !isCurrent {
			state.ID = id
		}

		states = append(states, wizardmodels.WizardStateMetadata{
			ID:        id,
			Comment:   state.Comment,
			CreatedAt: state.CreatedAt,
			UpdatedAt: state.UpdatedAt,
			IsCurrent: isCurrent,
		})
	}

	debuglog.DebugLog("ListWizardStates: found %d states", len(states))
	return states, nil
}

// DeleteWizardState удаляет состояние по ID.
func (ss *StateStore) DeleteWizardState(id string) error {
	if id == "" {
		return fmt.Errorf("state ID cannot be empty")
	}

	filePath := ss.getStateFilePath(id)

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("state file not found: %s", filePath)
	}

	// Удаляем файл
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete state file: %w", err)
	}

	debuglog.InfoLog("DeleteWizardState: deleted state file %s", filePath)
	return nil
}

// StateExists проверяет, существует ли состояние с указанным ID.
func (ss *StateStore) StateExists(id string) bool {
	filePath := ss.getStateFilePath(id)
	_, err := os.Stat(filePath)
	return err == nil
}

