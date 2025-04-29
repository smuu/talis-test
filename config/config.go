package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/celestiaorg/talis/pkg/models"
)

// expandPath expands $HOME and ~ in the given path
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return os.ExpandEnv(path)
}

// Config holds the application configuration
type Config struct {
	BaseURL            string
	Username           string
	ProjectName        string
	ProjectDescription string
	InstanceCount      int
	InstanceConfig     InstanceConfig
	SSHUsername        string
	SSHPrivateKeyPath  string
}

// InstanceConfig holds the configuration for creating instances
type InstanceConfig struct {
	Provider     models.ProviderID
	Region       string
	Size         string
	Image        string
	Tags         []string
	SSHKeyName   string
	SSHKeyPath   string
	VolumeConfig VolumeConfig
}

// VolumeConfig holds the configuration for instance volumes
type VolumeConfig struct {
	Name       string
	SizeGB     int
	MountPoint string
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	cfg := Config{
		BaseURL:            "http://localhost:8080",
		Username:           "test",
		ProjectName:        "test",
		ProjectDescription: "test",
		InstanceCount:      2,
		SSHUsername:        "root",
		SSHPrivateKeyPath:  "~/.ssh/digitalocean",
		InstanceConfig: InstanceConfig{
			Provider:   models.ProviderID("do"),
			Region:     "nyc1",
			Size:       "s-1vcpu-1gb",
			Image:      "ubuntu-22-04-x64",
			Tags:       []string{"talis", "dev", "testing"},
			SSHKeyName: "smuu",
			SSHKeyPath: "~/.ssh/digitalocean",
			VolumeConfig: VolumeConfig{
				Name:       "talis-volume",
				SizeGB:     15,
				MountPoint: "/mnt/data",
			},
		},
	}

	// Expand paths
	cfg.SSHPrivateKeyPath = expandPath(cfg.SSHPrivateKeyPath)
	cfg.InstanceConfig.SSHKeyPath = expandPath(cfg.InstanceConfig.SSHKeyPath)

	return cfg
}
