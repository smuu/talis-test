package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// InstanceInfo represents information about an instance
type InstanceInfo struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	PublicIP string `json:"public_ip"`
}

// State represents the persisted state of the application
type State struct {
	UserID    uint                      `json:"user_id"`
	Projects  map[string]uint           `json:"projects"`  // Map of project name to project ID
	Instances map[string][]InstanceInfo `json:"instances"` // Map of project name to instance info
}

// getStatePath returns the path to the state file
func getStatePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".talis-test", "state.json"), nil
}

// SaveState saves the current state to a file
func (m *TalisManager) SaveState(state State) error {
	statePath, err := getStatePath()
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statePath, data, 0644)
}

// LoadState loads the state from a file
func (m *TalisManager) LoadState() (State, error) {
	statePath, err := getStatePath()
	if err != nil {
		return State{}, err
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return State{
				Projects:  make(map[string]uint),
				Instances: make(map[string][]InstanceInfo),
			}, nil
		}
		return State{}, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}

	// Initialize maps if they're nil (for backward compatibility)
	if state.Projects == nil {
		state.Projects = make(map[string]uint)
	}
	if state.Instances == nil {
		state.Instances = make(map[string][]InstanceInfo)
	}

	return state, nil
}
