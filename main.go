package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/celestiaorg/talis-test/config"
	"github.com/celestiaorg/talis-test/manager"
	"github.com/joho/godotenv"
)

// NodeType represents the type of Celestia node to deploy
type NodeType string

const (
	ValidatorNode NodeType = "validator"
	BridgeNode    NodeType = "bridge"
	LightNode     NodeType = "light"
	FullNode      NodeType = "full"
)

// NodeConfig holds the configuration for a specific node
type NodeConfig struct {
	Type       NodeType
	Count      int
	Region     string
	Size       string
	VolumeSize int
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Parse command line flags
	deleteFlag := flag.Bool("delete", false, "Delete all deployed instances")
	setupNetworkFlag := flag.Bool("setup-network", false, "Set up Celestia network on deployed instances")
	chainIDFlag := flag.String("chain-id", "test-chain", "Chain ID for the Celestia network")
	flag.Parse()

	// Define your deployment configuration here
	deployment := struct {
		Nodes []NodeConfig
	}{
		Nodes: []NodeConfig{
			{
				Type:       ValidatorNode,
				Count:      4,
				Region:     "nyc1",
				Size:       "s-2vcpu-4gb",
				VolumeSize: 30,
			},
			{
				Type:       BridgeNode,
				Count:      0,
				Region:     "fra1",
				Size:       "s-4vcpu-8gb",
				VolumeSize: 50,
			},
		},
	}

	// Get configuration based on deployment specification
	cfg := getConfiguration(deployment.Nodes)

	// Create manager
	mgr, err := manager.NewTalisManager(cfg)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	// If delete flag is set, only delete instances and exit
	if *deleteFlag {
		log.Println("Deleting all instances...")
		if err := mgr.DeleteAllInstances(ctx); err != nil {
			log.Fatalf("Failed to delete instances: %v", err)
		}
		log.Println("Instance deletion completed successfully")
		return
	}

	// If setup-network flag is set, set up the Celestia network and exit
	if *setupNetworkFlag {
		log.Println("Setting up Celestia network...")
		if err := mgr.SetupCelestiaNetwork(ctx, *chainIDFlag); err != nil {
			log.Fatalf("Failed to set up Celestia network: %v", err)
		}
		log.Println("Celestia network setup completed successfully")
		return
	}

	// Define which actions to perform
	// Set these to true/false based on what you want to do
	runPrepare := true
	runInstallGo := true
	runInstallCelestiaApp := true
	runInstallCelestiaNode := true

	// Run the preparation stage if configured
	if runPrepare {
		log.Println("Preparing infrastructure...")
		if err := mgr.PrepareInfrastructure(ctx); err != nil {
			log.Fatalf("Failed to prepare infrastructure: %v", err)
		}
		log.Println("Infrastructure preparation completed successfully")
	}

	// Run the Go installation stage if configured
	if runInstallGo {
		log.Println("Installing Go on instances...")
		if err := mgr.InstallGoOnInstances(ctx); err != nil {
			log.Fatalf("Failed to install Go on instances: %v", err)
		}
		log.Println("Go installation completed successfully")
	}

	// Run the Celestia App installation stage if configured
	if runInstallCelestiaApp {
		log.Println("Installing Celestia App on configured instances...")
		if err := mgr.InstallCelestiaAppOnInstances(ctx); err != nil {
			log.Fatalf("Failed to install Celestia App on instances: %v", err)
		}
		log.Println("Celestia App installation completed successfully")
	}

	// Run the Celestia Node installation stage if configured
	if runInstallCelestiaNode {
		log.Println("Installing Celestia Node on configured instances...")
		if err := mgr.InstallCelestiaNodeOnInstances(ctx); err != nil {
			log.Fatalf("Failed to install Celestia Node on instances: %v", err)
		}
		log.Println("Celestia Node installation completed successfully")
	}
}

// getConfiguration returns the configuration for the application
// This is where you define your instances and their configurations
func getConfiguration(nodes []NodeConfig) config.Config {
	// Start with default configuration
	cfg := config.DefaultConfig()

	// Clear default instances
	cfg.Instances = []config.InstanceDefinition{}

	// Create instances based on node configuration
	for _, nodeConfig := range nodes {
		for i := 1; i <= nodeConfig.Count; i++ {
			// Determine which components to install based on node type
			installApp := false
			installNode := false

			switch nodeConfig.Type {
			case ValidatorNode:
				installApp = true
			case BridgeNode:
				installNode = true
			case LightNode:
				installNode = true
			case FullNode:
				installApp = true
				installNode = true
			}

			// Create the instance definition
			instance := config.NewInstanceDefinition(
				string(nodeConfig.Type)+"-"+fmt.Sprintf("%d", i)+"-"+fmt.Sprint(time.Now().Unix()),
				installApp,
				installNode,
			).
				WithRegion(nodeConfig.Region).
				WithSize(nodeConfig.Size).
				WithVolumeSize(nodeConfig.VolumeSize)

			// Add instance to configuration
			cfg.Instances = append(cfg.Instances, instance)
		}
	}

	return cfg
}
