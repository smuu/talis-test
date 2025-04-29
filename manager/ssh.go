package manager

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
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

// SSHConfig holds SSH configuration
type SSHConfig struct {
	Username   string
	PrivateKey string
}

// SSHManager handles SSH operations
type SSHManager struct {
	config SSHConfig
}

// NewSSHManager creates a new SSHManager instance
func NewSSHManager(config SSHConfig) *SSHManager {
	// Expand the private key path
	config.PrivateKey = expandPath(config.PrivateKey)
	return &SSHManager{
		config: config,
	}
}

// ExecuteCommand executes a command on a remote server via SSH
func (s *SSHManager) ExecuteCommand(host string, command string) error {
	// Read private key
	key, err := os.ReadFile(s.config.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to read private key from %s: %w", s.config.PrivateKey, err)
	}

	// Create signer
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// SSH client config
	config := &ssh.ClientConfig{
		User: s.config.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: In production, use proper host key verification
	}

	// Connect to server
	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	defer client.Close()

	// Create session
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Execute command
	output, err := session.CombinedOutput(command)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w, output: %s", err, output)
	}

	log.Printf("Command output on %s: %s", host, output)
	return nil
}

// WriteToFile writes content to a file on a remote server
func (s *SSHManager) WriteToFile(host, path, content string) error {
	command := fmt.Sprintf("echo '%s' > %s", content, path)
	return s.ExecuteCommand(host, command)
}
