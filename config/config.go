package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/celestiaorg/talis/pkg/db/models"
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

// ProviderFromString converts a string to a ProviderID
func ProviderFromString(provider string) models.ProviderID {
	return models.ProviderID(provider)
}

// Config holds the application configuration
type Config struct {
	BaseURL             string
	APIKey              string
	Username            string
	ProjectName         string
	ProjectDescription  string
	Instances           []InstanceDefinition
	SSHUsername         string
	SSHPrivateKeyPath   string
	GoVersion           string
	CelestiaAppVersion  string
	CelestiaNodeVersion string
}

// InstanceDefinition defines a single instance with its configuration
type InstanceDefinition struct {
	Name                string
	InstanceConfig      InstanceConfig
	InstallCelestiaApp  bool
	InstallCelestiaNode bool
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

// NewInstanceDefinition creates a new instance definition with default values
func NewInstanceDefinition(name string, installApp, installNode bool) InstanceDefinition {
	return InstanceDefinition{
		Name:                name,
		InstallCelestiaApp:  installApp,
		InstallCelestiaNode: installNode,
		InstanceConfig: InstanceConfig{
			Provider:   models.ProviderID("do"),
			Region:     "nyc1",
			Size:       "s-1vcpu-1gb",
			Image:      "ubuntu-24-04-x64",
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
}

// WithRegion sets the region for the instance
func (i InstanceDefinition) WithRegion(region string) InstanceDefinition {
	i.InstanceConfig.Region = region
	return i
}

// WithSize sets the size for the instance
func (i InstanceDefinition) WithSize(size string) InstanceDefinition {
	i.InstanceConfig.Size = size
	return i
}

// WithVolumeSize sets the volume size for the instance
func (i InstanceDefinition) WithVolumeSize(sizeGB int) InstanceDefinition {
	i.InstanceConfig.VolumeConfig.SizeGB = sizeGB
	return i
}

// WithProvider sets the provider for the instance
func (i InstanceDefinition) WithProvider(provider string) InstanceDefinition {
	i.InstanceConfig.Provider = ProviderFromString(provider)
	return i
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	cfg := Config{
		BaseURL:             "http://163.172.162.109:8000/talis/",
		APIKey:              os.Getenv("TALIS_KEY"),
		Username:            "smuu",
		ProjectName:         "smuu",
		ProjectDescription:  "smuus project",
		SSHUsername:         "root",
		SSHPrivateKeyPath:   "~/.ssh/digitalocean",
		GoVersion:           "1.23.0",
		CelestiaAppVersion:  "v3.4.2-mammoth-v0.7.0",
		CelestiaNodeVersion: "v0.21.9-mammoth-v0.0.16",
		Instances: []InstanceDefinition{
			NewInstanceDefinition("default", true, false),
		},
	}

	// Expand paths
	cfg.SSHPrivateKeyPath = expandPath(cfg.SSHPrivateKeyPath)
	for i := range cfg.Instances {
		cfg.Instances[i].InstanceConfig.SSHKeyPath = expandPath(cfg.Instances[i].InstanceConfig.SSHKeyPath)
	}

	return cfg
}
