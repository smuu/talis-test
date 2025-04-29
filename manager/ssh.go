package manager

import (
	"bytes"
	"fmt"
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

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Build command that sources profile files if they exist
	cmd := fmt.Sprintf(`
if [ -f "$HOME/.bashrc" ]; then
    source "$HOME/.bashrc"
elif [ -f "$HOME/.bash_profile" ]; then
    source "$HOME/.bash_profile"
fi

# Ensure Go is in PATH
if [ -d "/usr/local/go/bin" ]; then
    export PATH=$PATH:/usr/local/go/bin
fi
if [ -d "$HOME/go/bin" ]; then
    export PATH=$PATH:$HOME/go/bin
fi

%s`, command)

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to execute command: %w\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

// WriteToFile writes content to a file on a remote server
func (s *SSHManager) WriteToFile(host, path, content string) error {
	// Escape single quotes in content
	escapedContent := strings.ReplaceAll(content, "'", "'\"'\"'")
	command := fmt.Sprintf("echo '%s' > %s", escapedContent, path)

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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Execute command with profile sourcing
	cmd := fmt.Sprintf(`
if [ -f "$HOME/.bashrc" ]; then
    source "$HOME/.bashrc"
elif [ -f "$HOME/.bash_profile" ]; then
    source "$HOME/.bash_profile"
fi

# Ensure Go is in PATH
if [ -d "/usr/local/go/bin" ]; then
    export PATH=$PATH:/usr/local/go/bin
fi
if [ -d "$HOME/go/bin" ]; then
    export PATH=$PATH:$HOME/go/bin
fi

%s`, command)

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to execute command: %w\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

// CopyFile copies a local file to a remote machine
func (s *SSHManager) CopyFile(host, localPath, remotePath string) error {
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

	// Read the local file
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file %s: %w", localPath, err)
	}

	// Create a temporary file on the remote machine
	tempPath := fmt.Sprintf("/tmp/%s", filepath.Base(localPath))
	if err := s.WriteToFile(host, tempPath, string(content)); err != nil {
		return fmt.Errorf("failed to write temporary file on remote machine: %w", err)
	}

	// Move the temporary file to the final destination
	if err := s.ExecuteCommand(host, fmt.Sprintf("mv %s %s", tempPath, remotePath)); err != nil {
		return fmt.Errorf("failed to move file to final destination: %w", err)
	}

	return nil
}
