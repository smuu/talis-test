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
	infraFlag := flag.Bool("infra", false, "Create infrastructure (servers with Talis)")
	prepareToolsFlag := flag.Bool("prepare-tools", false, "Install required tools (Go, Celestia)")
	prepareChainFlag := flag.Bool("prepare-chain", false, "Create and add chain files")
	startFlag := flag.Bool("start", false, "Start the validators")
	deleteFlag := flag.Bool("delete", false, "Delete all deployed instances")
	chainIDFlag := flag.String("chain-id", "test-chain", "Chain ID for the Celestia network")
	flag.Parse()

	// Define your deployment configuration here
	deployment := struct {
		Nodes []NodeConfig
	}{
		Nodes: []NodeConfig{
			{
				Type:       ValidatorNode,
				Count:      21,
				Region:     "nyc1",
				Size:       "s-2vcpu-4gb",
				VolumeSize: 30,
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

	// Run infrastructure setup if requested
	if *infraFlag {
		log.Println("Preparing infrastructure...")
		if err := mgr.PrepareInfrastructure(ctx); err != nil {
			log.Fatalf("Failed to prepare infrastructure: %v", err)
		}
		log.Println("Infrastructure preparation completed successfully")
	}

	// Run tools installation if requested
	if *prepareToolsFlag {
		log.Println("Installing required tools...")

		// Install Go
		log.Println("Installing Go on instances...")
		if err := mgr.InstallGoOnInstances(ctx); err != nil {
			log.Fatalf("Failed to install Go on instances: %v", err)
		}
		log.Println("Go installation completed successfully")

		// Install Celestia App
		log.Println("Installing Celestia App on configured instances...")
		if err := mgr.InstallCelestiaAppOnInstances(ctx); err != nil {
			log.Fatalf("Failed to install Celestia App on instances: %v", err)
		}
		log.Println("Celestia App installation completed successfully")

		// Install Celestia Node
		log.Println("Installing Celestia Node on configured instances...")
		if err := mgr.InstallCelestiaNodeOnInstances(ctx); err != nil {
			log.Fatalf("Failed to install Celestia Node on instances: %v", err)
		}
		log.Println("Celestia Node installation completed successfully")
	}

	// Run chain preparation if requested
	if *prepareChainFlag {
		log.Println("Setting up Celestia network...")
		if err := mgr.SetupCelestiaNetwork(ctx, *chainIDFlag); err != nil {
			log.Fatalf("Failed to set up Celestia network: %v", err)
		}
		log.Println("Celestia network setup completed successfully")
	}

	// Run validator start if requested
	if *startFlag {
		log.Println("Starting Celestia App service on configured instances...")
		if err := mgr.SetupCelestiaAppService(ctx); err != nil {
			log.Fatalf("Failed to start Celestia App service: %v", err)
		}
		log.Println("Celestia App service started successfully")
	}

	// If no flags are set, show usage
	if !*infraFlag && !*prepareToolsFlag && !*prepareChainFlag && !*startFlag && !*deleteFlag {
		fmt.Println("No action specified. Use one of the following flags:")
		fmt.Println("  -infra         Create infrastructure (servers with Talis)")
		fmt.Println("  -prepare-tools Install required tools (Go, Celestia)")
		fmt.Println("  -prepare-chain Create and add chain files")
		fmt.Println("  -start         Start the validators")
		fmt.Println("  -delete        Delete all deployed instances")
		fmt.Println("\nAdditional options:")
		fmt.Println("  -chain-id      Chain ID for the Celestia network (default: test-chain)")
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
