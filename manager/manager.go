package manager

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/celestiaorg/talis-test/config"
	"github.com/celestiaorg/talis/pkg/api/v1/client"
	"github.com/celestiaorg/talis/pkg/api/v1/handlers"
	"github.com/celestiaorg/talis/pkg/models"
	"github.com/celestiaorg/talis/pkg/types"
)

// TalisManager manages the Talis client and operations
type TalisManager struct {
	client     client.Client
	config     config.Config
	state      State
	sshManager *SSHManager
}

// NewTalisManager creates a new TalisManager instance
func NewTalisManager(config config.Config) (*TalisManager, error) {
	opts := client.DefaultOptions()
	opts.BaseURL = config.BaseURL

	client, err := client.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	sshManager := NewSSHManager(SSHConfig{
		Username:   config.SSHUsername,
		PrivateKey: config.SSHPrivateKeyPath,
	})

	return &TalisManager{
		client:     client,
		config:     config,
		sshManager: sshManager,
	}, nil
}

// PrepareInfrastructure sets up the required infrastructure
func (m *TalisManager) PrepareInfrastructure(ctx context.Context) error {
	// Load existing state
	state, err := m.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	m.state = state

	// Create user if not exists
	userID := state.UserID
	if userID == 0 {
		userID, err = m.createUserIfNotExists(ctx)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
		state.UserID = userID
		if err := m.SaveState(state); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}
	}

	// Create project if not exists
	projectID := state.Projects[m.config.ProjectName]
	if projectID == 0 {
		projectID, err = m.createProjectIfNotExists(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}
		state.Projects[m.config.ProjectName] = projectID
		if err := m.SaveState(state); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}
	}

	// Create instances
	instanceIDs, err := m.createInstances(ctx, userID, projectID)
	if err != nil {
		return fmt.Errorf("failed to create instances: %w", err)
	}

	// Wait for instances to be ready
	if err := m.waitForInstancesToBeReady(ctx, instanceIDs, 5*time.Minute); err != nil {
		return fmt.Errorf("failed to wait for instances: %w", err)
	}

	// Get IPs of instances
	for _, instanceID := range instanceIDs {
		instance, err := m.client.GetInstance(ctx, strconv.Itoa(int(instanceID)))
		if err != nil {
			return fmt.Errorf("failed to get instance %d: %w", instanceID, err)
		}
		log.Printf("Instance %d IP: %s", instanceID, instance.PublicIP)

		// Update IP in state
		for i, inst := range m.state.Instances[m.config.ProjectName] {
			if inst.ID == instanceID {
				m.state.Instances[m.config.ProjectName][i].PublicIP = instance.PublicIP
				break
			}
		}
	}

	// Save state with updated IPs
	if err := m.SaveState(m.state); err != nil {
		return fmt.Errorf("failed to save state with IPs: %w", err)
	}

	return nil
}

// InstallGoOnInstances installs Go on all instances if not already installed
func (m *TalisManager) InstallGoOnInstances(ctx context.Context) error {
	// Load state
	state, err := m.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	m.state = state

	// Create a semaphore to limit concurrent installations
	sem := make(chan struct{}, 10)
	errChan := make(chan error, len(m.state.Instances[m.config.ProjectName]))
	var wg sync.WaitGroup

	// For each instance, check and install Go if needed
	for _, instance := range m.state.Instances[m.config.ProjectName] {
		if instance.PublicIP == "" {
			log.Printf("Skipping instance %d: no public IP", instance.ID)
			continue
		}

		wg.Add(1)
		go func(inst InstanceInfo) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			log.Printf("Checking Go installation on instance %s...", inst.PublicIP)

			// Check if Go is installed in common locations
			checkCmd := `
if [ -x "/usr/local/go/bin/go" ] || [ -x "$HOME/go/bin/go" ] || command -v go > /dev/null 2>&1; then
    echo "Go is installed"
    exit 0
else
    echo "Go is not installed"
    exit 1
fi`
			err := m.sshManager.ExecuteCommand(inst.PublicIP, checkCmd)
			if err == nil {
				log.Printf("Go is already installed on instance %s", inst.PublicIP)
				return
			}

			log.Printf("Installing Go on instance %s...", inst.PublicIP)

			// Copy the installation script to the remote machine
			if err := m.sshManager.CopyFile(inst.PublicIP, "scripts/install_go.sh", "install_go.sh"); err != nil {
				errChan <- fmt.Errorf("failed to copy installation script to instance %s: %w", inst.PublicIP, err)
				return
			}

			// Make the script executable and run it
			if err := m.sshManager.ExecuteCommand(inst.PublicIP, fmt.Sprintf("chmod +x install_go.sh && ./install_go.sh %s", m.config.GoVersion)); err != nil {
				errChan <- fmt.Errorf("failed to execute Go installation script on instance %s: %w", inst.PublicIP, err)
				return
			}

			log.Printf("Successfully installed Go and required packages on instance %s", inst.PublicIP)
		}(instance)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// InstallCelestiaAppOnInstances installs Celestia App on all instances
func (m *TalisManager) InstallCelestiaAppOnInstances(ctx context.Context) error {
	// Load state
	state, err := m.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	m.state = state

	// Create a semaphore to limit concurrent installations
	sem := make(chan struct{}, 10)
	errChan := make(chan error, len(m.state.Instances[m.config.ProjectName]))
	var wg sync.WaitGroup

	// For each instance, install Celestia App
	for _, instance := range m.state.Instances[m.config.ProjectName] {
		if instance.PublicIP == "" {
			log.Printf("Skipping instance %d: no public IP", instance.ID)
			continue
		}

		wg.Add(1)
		go func(inst InstanceInfo) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check if Celestia App is already installed
			log.Printf("Checking Celestia App installation on instance %s...", inst.PublicIP)
			checkCmd := `
if [ -x "/usr/local/bin/celestia-appd" ] || [ -x "$HOME/go/bin/celestia-appd" ] || [ -x "/root/celestia-app-temp/celestia-appd" ] || command -v celestia-appd > /dev/null 2>&1; then
    echo "Celestia App is installed"
    exit 0
else
    echo "Celestia App is not installed"
    exit 1
fi`
			err := m.sshManager.ExecuteCommand(inst.PublicIP, checkCmd)
			if err == nil {
				log.Printf("Celestia App is already installed on instance %s", inst.PublicIP)
				return
			}

			log.Printf("Installing Celestia App on instance %s...", inst.PublicIP)

			// Copy the installation script to the remote machine
			if err := m.sshManager.CopyFile(inst.PublicIP, "scripts/install_celestia_app.sh", "install_celestia_app.sh"); err != nil {
				errChan <- fmt.Errorf("failed to copy installation script to instance %s: %w", inst.PublicIP, err)
				return
			}

			// Make the script executable and run it
			if err := m.sshManager.ExecuteCommand(inst.PublicIP, fmt.Sprintf("chmod +x install_celestia_app.sh && ./install_celestia_app.sh %s", m.config.CelestiaAppVersion)); err != nil {
				errChan <- fmt.Errorf("failed to execute Celestia App installation script on instance %s: %w", inst.PublicIP, err)
				return
			}

			log.Printf("Successfully installed Celestia App on instance %s", inst.PublicIP)
		}(instance)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// InstallCelestiaNodeOnInstances installs Celestia Node on all instances
func (m *TalisManager) InstallCelestiaNodeOnInstances(ctx context.Context) error {
	// Load state
	state, err := m.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	m.state = state

	// Create a semaphore to limit concurrent installations
	sem := make(chan struct{}, 10)
	errChan := make(chan error, len(m.state.Instances[m.config.ProjectName]))
	var wg sync.WaitGroup

	// For each instance, install Celestia Node
	for _, instance := range m.state.Instances[m.config.ProjectName] {
		if instance.PublicIP == "" {
			log.Printf("Skipping instance %d: no public IP", instance.ID)
			continue
		}

		wg.Add(1)
		go func(inst InstanceInfo) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check if Celestia Node is already installed
			log.Printf("Checking Celestia Node installation on instance %s...", inst.PublicIP)
			checkCmd := `
if [ -x "/usr/local/bin/celestia" ] || [ -x "$HOME/go/bin/celestia" ] || [ -x "/root/celestia-node-temp/celestia" ] || command -v celestia > /dev/null 2>&1; then
    echo "Celestia Node is installed"
    exit 0
else
    echo "Celestia Node is not installed"
    exit 1
fi`
			err := m.sshManager.ExecuteCommand(inst.PublicIP, checkCmd)
			if err == nil {
				log.Printf("Celestia Node is already installed on instance %s", inst.PublicIP)
				return
			}

			log.Printf("Installing Celestia Node on instance %s...", inst.PublicIP)

			// Copy the installation script to the remote machine
			if err := m.sshManager.CopyFile(inst.PublicIP, "scripts/install_celestia_node.sh", "install_celestia_node.sh"); err != nil {
				errChan <- fmt.Errorf("failed to copy installation script to instance %s: %w", inst.PublicIP, err)
				return
			}

			// Make the script executable and run it
			if err := m.sshManager.ExecuteCommand(inst.PublicIP, fmt.Sprintf("chmod +x install_celestia_node.sh && ./install_celestia_node.sh %s", m.config.CelestiaNodeVersion)); err != nil {
				errChan <- fmt.Errorf("failed to execute Celestia Node installation script on instance %s: %w", inst.PublicIP, err)
				return
			}

			log.Printf("Successfully installed Celestia Node on instance %s", inst.PublicIP)
		}(instance)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// Run executes all stages of the workflow
func (m *TalisManager) Run(ctx context.Context) error {
	// Stage 1: Prepare infrastructure
	if err := m.PrepareInfrastructure(ctx); err != nil {
		return fmt.Errorf("failed to prepare infrastructure: %w", err)
	}

	// Stage 2: Install Go on instances
	if err := m.InstallGoOnInstances(ctx); err != nil {
		return fmt.Errorf("failed to install Go on instances: %w", err)
	}

	// Stage 3: Install Celestia App on instances
	if err := m.InstallCelestiaAppOnInstances(ctx); err != nil {
		return fmt.Errorf("failed to install Celestia App on instances: %w", err)
	}

	// Stage 4: Install Celestia Node on instances
	if err := m.InstallCelestiaNodeOnInstances(ctx); err != nil {
		return fmt.Errorf("failed to install Celestia Node on instances: %w", err)
	}

	return nil
}

// createUserIfNotExists creates a user if it doesn't exist
func (m *TalisManager) createUserIfNotExists(ctx context.Context) (uint, error) {
	users, err := m.client.GetUsers(ctx, handlers.UserGetParams{
		Username: m.config.Username,
	})
	if err != nil {
		// Check if the error contains a 404 status code
		if strings.Contains(err.Error(), "\"code\":404") {
			user, err := m.client.CreateUser(ctx, handlers.CreateUserParams{
				Username: m.config.Username,
			})
			if err != nil {
				return 0, fmt.Errorf("failed to create user: %w", err)
			}
			return user.UserID, nil
		}
		return 0, fmt.Errorf("failed to get users: %w", err)
	}

	return users.User.ID, nil
}

// createProjectIfNotExists creates a project if it doesn't exist
func (m *TalisManager) createProjectIfNotExists(ctx context.Context, userID uint) (uint, error) {
	project, err := m.client.GetProject(ctx, handlers.ProjectGetParams{
		Name:    m.config.ProjectName,
		OwnerID: userID,
	})
	if err != nil {
		// Check if the error contains a 404 status code
		if strings.Contains(err.Error(), "\"code\":404") {
			project, err := m.client.CreateProject(ctx, handlers.ProjectCreateParams{
				Name:        m.config.ProjectName,
				Description: m.config.ProjectDescription,
				OwnerID:     userID,
			})
			if err != nil {
				return 0, fmt.Errorf("failed to create project: %w", err)
			}
			return project.ID, nil
		}
		return 0, fmt.Errorf("failed to get project: %w", err)
	}

	return project.ID, nil
}

// createInstances creates the specified number of instances
func (m *TalisManager) createInstances(ctx context.Context, userID, projectID uint) ([]uint, error) {
	instanceIDs := make([]uint, 0, m.config.InstanceCount)
	existingInstances := m.state.Instances[m.config.ProjectName]

	// Create only the instances that don't exist yet
	for i := 0; i < m.config.InstanceCount; i++ {
		instanceName := fmt.Sprintf("%s-%d", m.config.ProjectName, i)

		// Check if instance already exists
		exists := false
		for _, instance := range existingInstances {
			if instance.Name == instanceName {
				instanceIDs = append(instanceIDs, instance.ID)
				exists = true
				break
			}
		}

		if !exists {
			instanceID, err := m.createInstance(ctx, userID, projectID, i)
			if err != nil {
				return nil, fmt.Errorf("failed to create instance %d: %w", i+1, err)
			}
			instanceIDs = append(instanceIDs, instanceID)

			// Add new instance to state
			existingInstances = append(existingInstances, InstanceInfo{
				ID:   instanceID,
				Name: instanceName,
			})
		}
	}

	// Update state with all instances
	m.state.Instances[m.config.ProjectName] = existingInstances
	if err := m.SaveState(m.state); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	return instanceIDs, nil
}

// createInstance creates a single instance
func (m *TalisManager) createInstance(ctx context.Context, userID, projectID uint, instanceIndex int) (uint, error) {
	err := m.client.CreateInstance(ctx, []types.InstanceRequest{
		{
			Name:              fmt.Sprintf("%s-%d", m.config.ProjectName, instanceIndex),
			OwnerID:           userID,
			ProjectID:         projectID,
			Provider:          m.config.InstanceConfig.Provider,
			NumberOfInstances: 1,
			Provision:         false,
			Region:            m.config.InstanceConfig.Region,
			Size:              m.config.InstanceConfig.Size,
			Image:             m.config.InstanceConfig.Image,
			Tags:              m.config.InstanceConfig.Tags,
			SSHKeyName:        m.config.InstanceConfig.SSHKeyName,
			SSHKeyPath:        m.config.InstanceConfig.SSHKeyPath,
			Volumes: []types.VolumeConfig{
				{
					Name:       m.config.InstanceConfig.VolumeConfig.Name,
					SizeGB:     m.config.InstanceConfig.VolumeConfig.SizeGB,
					MountPoint: m.config.InstanceConfig.VolumeConfig.MountPoint,
				},
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create instance: %w", err)
	}

	pendingInstances, err := m.getPendingInstances(ctx, userID, projectID)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending instances: %w", err)
	}

	if len(pendingInstances) == 0 {
		return 0, fmt.Errorf("no pending instances found")
	}

	// Return the most recently created instance
	mostRecent := pendingInstances[0]
	for _, instance := range pendingInstances {
		if instance.CreatedAt.After(mostRecent.CreatedAt) {
			mostRecent = instance
		}
	}

	return mostRecent.ID, nil
}

// getPendingInstances retrieves all pending instances
func (m *TalisManager) getPendingInstances(ctx context.Context, userID, projectID uint) ([]models.Instance, error) {
	instances, err := m.client.ListProjectInstances(ctx, handlers.ProjectListInstancesParams{
		ProjectID: projectID,
		OwnerID:   userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list project instances: %w", err)
	}

	pendingInstances := make([]models.Instance, 0)
	for _, instance := range instances {
		if instance.Status == models.InstanceStatusPending || instance.Status == models.InstanceStatusProvisioning {
			pendingInstances = append(pendingInstances, instance)
		}
	}

	return pendingInstances, nil
}

// waitForInstancesToBeReady waits for all instances to be ready
func (m *TalisManager) waitForInstancesToBeReady(ctx context.Context, instanceIDs []uint, timeout time.Duration) error {
	startTime := time.Now()
	for {
		allReady := true
		for _, instanceID := range instanceIDs {
			instance, err := m.client.GetInstance(ctx, strconv.Itoa(int(instanceID)))
			if err != nil {
				return fmt.Errorf("failed to get instance %d: %w", instanceID, err)
			}

			log.Printf("Instance %d status: %s", instanceID, instance.Status)
			if instance.Status != models.InstanceStatusReady {
				allReady = false
				break
			}
		}

		if allReady {
			log.Println("All instances are ready!")
			return nil
		}

		if time.Since(startTime) > timeout {
			return fmt.Errorf("instances not ready after %v", timeout)
		}

		time.Sleep(5 * time.Second)
	}
}

// deleteInstances deletes all specified instances
func (m *TalisManager) deleteInstances(ctx context.Context, userID, projectID uint, instanceIDs []uint) error {
	projectInstances := m.state.Instances[m.config.ProjectName]
	remainingInstances := make([]InstanceInfo, 0, len(projectInstances))

	for _, instance := range projectInstances {
		shouldDelete := false
		for _, id := range instanceIDs {
			if instance.ID == id {
				shouldDelete = true
				break
			}
		}

		if shouldDelete {
			err := m.client.DeleteInstances(ctx, types.DeleteInstancesRequest{
				OwnerID:     userID,
				ProjectID:   projectID,
				InstanceIDs: []uint{instance.ID},
			})
			if err != nil {
				return fmt.Errorf("failed to delete instance %d: %w", instance.ID, err)
			}
		} else {
			remainingInstances = append(remainingInstances, instance)
		}
	}

	// Update state with remaining instances
	m.state.Instances[m.config.ProjectName] = remainingInstances
	if err := m.SaveState(m.state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// DeleteAllInstances deletes all instances for the current project
func (m *TalisManager) DeleteAllInstances(ctx context.Context) error {
	// Load state
	state, err := m.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}
	m.state = state

	// Get project ID
	projectID := state.Projects[m.config.ProjectName]
	if projectID == 0 {
		return fmt.Errorf("project %s not found", m.config.ProjectName)
	}

	// Get user ID
	userID := state.UserID
	if userID == 0 {
		return fmt.Errorf("user ID not found")
	}

	// Get all instance IDs for the project
	instanceIDs := make([]uint, 0, len(state.Instances[m.config.ProjectName]))
	for _, instance := range state.Instances[m.config.ProjectName] {
		instanceIDs = append(instanceIDs, instance.ID)
	}

	if len(instanceIDs) == 0 {
		log.Printf("No instances found for project %s", m.config.ProjectName)
		return nil
	}

	// Delete all instances
	if err := m.deleteInstances(ctx, userID, projectID, instanceIDs); err != nil {
		return fmt.Errorf("failed to delete instances: %w", err)
	}

	log.Printf("Successfully deleted all instances for project %s", m.config.ProjectName)
	return nil
}
