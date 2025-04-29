package main

import (
	"context"
	"flag"
	"log"

	"github.com/celestiaorg/talis-test/config"
	"github.com/celestiaorg/talis-test/manager"
)

func main() {
	// Parse command line flags
	prepare := flag.Bool("prepare", false, "Prepare infrastructure")
	install := flag.Bool("install", false, "Install Go on instances")
	installCelestia := flag.Bool("install-celestia", false, "Install Celestia App on instances")
	installNode := flag.Bool("install-node", false, "Install Celestia Node on instances")
	delete := flag.Bool("delete", false, "Delete all instances")
	flag.Parse()

	// If no flags are set, run all stages
	if !*prepare && !*install && !*installCelestia && !*installNode && !*delete {
		*prepare = true
		*install = true
		*installCelestia = true
		*installNode = true
	}

	// Get default configuration
	cfg := config.DefaultConfig()

	// Create manager
	mgr, err := manager.NewTalisManager(cfg)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	ctx := context.Background()

	// Run the preparation stage if requested
	if *prepare {
		log.Println("Preparing infrastructure...")
		if err := mgr.PrepareInfrastructure(ctx); err != nil {
			log.Fatalf("Failed to prepare infrastructure: %v", err)
		}
		log.Println("Infrastructure preparation completed successfully")
	}

	// Run the Go installation stage if requested
	if *install {
		log.Println("Installing Go on instances...")
		if err := mgr.InstallGoOnInstances(ctx); err != nil {
			log.Fatalf("Failed to install Go on instances: %v", err)
		}
		log.Println("Go installation completed successfully")
	}

	// Run the Celestia App installation stage if requested
	if *installCelestia {
		log.Println("Installing Celestia App on instances...")
		if err := mgr.InstallCelestiaAppOnInstances(ctx); err != nil {
			log.Fatalf("Failed to install Celestia App on instances: %v", err)
		}
		log.Println("Celestia App installation completed successfully")
	}

	// Run the Celestia Node installation stage if requested
	if *installNode {
		log.Println("Installing Celestia Node on instances...")
		if err := mgr.InstallCelestiaNodeOnInstances(ctx); err != nil {
			log.Fatalf("Failed to install Celestia Node on instances: %v", err)
		}
		log.Println("Celestia Node installation completed successfully")
	}

	// Delete instances if requested
	if *delete {
		log.Println("Deleting all instances...")
		if err := mgr.DeleteAllInstances(ctx); err != nil {
			log.Fatalf("Failed to delete instances: %v", err)
		}
		log.Println("Instance deletion completed successfully")
	}
}
