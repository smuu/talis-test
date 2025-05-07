package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/celestiaorg/celestia-app/v3/app"
	"github.com/celestiaorg/celestia-app/v3/test/util/genesis"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
)

// CelestiaNetwork represents a Celestia network configuration
type CelestiaNetwork struct {
	chainID    string
	genesis    *genesis.Genesis
	keygen     *keyGenerator
	nodes      []*CelestiaNode
	sshManager *SSHManager
}

// CelestiaNode represents a Celestia node in the network
type CelestiaNode struct {
	name       string
	signerKey  *keyPair
	networkKey *keyPair
	sshManager *SSHManager
	homeDir    string
	publicIP   string
}

// NewCelestiaNetwork creates a new Celestia network configuration
func NewCelestiaNetwork(chainID string, sshManager *SSHManager) *CelestiaNetwork {
	return &CelestiaNetwork{
		chainID:    chainID,
		genesis:    genesis.NewDefaultGenesis().WithChainID(chainID),
		keygen:     newKeyGenerator(42), // Use a fixed seed for reproducibility
		sshManager: sshManager,
		nodes:      make([]*CelestiaNode, 0),
	}
}

// CreateGenesisNode creates a new genesis validator node
func (n *CelestiaNetwork) CreateGenesisNode(ctx context.Context, name, homeDir, publicIP string) error {
	signerKey := n.keygen.Generate(ed25519Type)
	networkKey := n.keygen.Generate(ed25519Type)

	node := &CelestiaNode{
		name:       name,
		signerKey:  signerKey,
		networkKey: networkKey,
		sshManager: n.sshManager,
		homeDir:    homeDir,
		publicIP:   publicIP,
	}

	// Add validator to genesis
	if err := n.genesis.NewValidator(node.GenesisValidator()); err != nil {
		return fmt.Errorf("failed to add validator to genesis: %w", err)
	}

	n.nodes = append(n.nodes, node)
	return nil
}

// SetupNetwork sets up the Celestia network on the instances
func (n *CelestiaNetwork) SetupNetwork(ctx context.Context) error {
	fmt.Printf("Starting Celestia network setup with %d nodes...\n", len(n.nodes))

	// Create home directories and copy keys for each node
	for _, node := range n.nodes {
		fmt.Printf("Setting up node %s...\n", node.name)
		if err := node.setupNode(ctx); err != nil {
			return fmt.Errorf("failed to setup node %s: %w", node.name, err)
		}
		fmt.Printf("Node %s setup completed\n", node.name)
	}

	// Get peer addresses
	fmt.Println("Configuring peer connections...")
	peers := make([]string, 0, len(n.nodes))
	for _, node := range n.nodes {
		peers = append(peers, node.AddressP2P())
	}

	// Setup each node with configuration files
	for _, node := range n.nodes {
		fmt.Printf("Configuring node %s...\n", node.name)
		if err := node.setupConfig(peers); err != nil {
			return fmt.Errorf("failed to setup config for node %s: %w", node.name, err)
		}
		fmt.Printf("Node %s configuration completed\n", node.name)
	}

	// Export genesis file
	fmt.Println("Generating genesis file...")
	genesisDoc, err := n.genesis.Export()
	if err != nil {
		return fmt.Errorf("failed to export genesis: %w", err)
	}

	// To fix this issue:
	// Error: error reading GenesisDoc at /root/.celestia-app/config/genesis.json: block.MaxBytes is too big. 128000000 > 104857600
	genesisDoc.ConsensusParams.Block.MaxBytes = 104857600

	// Write genesis file to each node
	fmt.Println("Distributing genesis file to nodes...")
	for _, node := range n.nodes {
		// Remove existing genesis file if it exists
		remoteGenesisPath := filepath.Join(node.homeDir, "config", "genesis.json")
		if err := node.sshManager.ExecuteCommand(node.publicIP, fmt.Sprintf("rm -f %s", remoteGenesisPath)); err != nil {
			return fmt.Errorf("failed to remove existing genesis file: %w", err)
		}

		// Create a temporary directory for the genesis file
		tmpDir, err := os.MkdirTemp("", "celestia-genesis-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		// Write genesis file to temp directory
		genesisPath := filepath.Join(tmpDir, "genesis.json")
		if err := genesisDoc.SaveAs(genesisPath); err != nil {
			return fmt.Errorf("failed to save genesis file: %w", err)
		}

		// Read the genesis file content
		genesisContent, err := os.ReadFile(genesisPath)
		if err != nil {
			return fmt.Errorf("failed to read genesis file: %w", err)
		}

		// Write genesis file to remote node
		if err := node.sshManager.WriteToFile(node.publicIP, remoteGenesisPath, string(genesisContent)); err != nil {
			return fmt.Errorf("failed to write genesis file to node %s: %w", node.name, err)
		}
		// Set correct permissions for genesis.json
		if err := node.sshManager.ExecuteCommand(node.publicIP, fmt.Sprintf("chmod 644 %s", remoteGenesisPath)); err != nil {
			return fmt.Errorf("failed to set permissions for genesis file: %w", err)
		}
		fmt.Printf("Genesis file written to node %s\n", node.name)
	}

	fmt.Println("Celestia network setup completed successfully")
	return nil
}

// setupNode sets up a single Celestia node
func (n *CelestiaNode) setupNode(ctx context.Context) error {
	// Create config and data directories
	fmt.Printf("Creating directories for node %s...\n", n.name)
	for _, dir := range []string{
		filepath.Join(n.homeDir, "config"),
		filepath.Join(n.homeDir, "data"),
	} {
		if err := n.sshManager.ExecuteCommand(n.publicIP, fmt.Sprintf("mkdir -p %s", dir)); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Remove existing key files if they exist
	keyFiles := []string{
		filepath.Join(n.homeDir, "config", "priv_validator_key.json"),
		filepath.Join(n.homeDir, "data", "priv_validator_state.json"),
		filepath.Join(n.homeDir, "config", "node_key.json"),
	}
	for _, file := range keyFiles {
		if err := n.sshManager.ExecuteCommand(n.publicIP, fmt.Sprintf("rm -f %s", file)); err != nil {
			return fmt.Errorf("failed to remove existing file %s: %w", file, err)
		}
	}

	// Copy node keys
	fmt.Printf("Setting up keys for node %s...\n", n.name)
	if err := n.copyNodeKeys(ctx); err != nil {
		return fmt.Errorf("failed to copy node keys: %w", err)
	}

	return nil
}

// setupConfig sets up the configuration files for a Celestia node
func (n *CelestiaNode) setupConfig(peers []string) error {
	fmt.Printf("Creating configuration files for node %s...\n", n.name)

	// Remove existing config files if they exist
	configFiles := []string{
		filepath.Join(n.homeDir, "config", "config.toml"),
		filepath.Join(n.homeDir, "config", "app.toml"),
	}
	for _, file := range configFiles {
		if err := n.sshManager.ExecuteCommand(n.publicIP, fmt.Sprintf("rm -f %s", file)); err != nil {
			return fmt.Errorf("failed to remove existing config file %s: %w", file, err)
		}
	}

	// Create config.toml
	cfg := app.DefaultConsensusConfig()
	cfg.TxIndex.Indexer = "kv"
	cfg.Consensus.TimeoutPropose = config.DefaultConsensusConfig().TimeoutPropose
	cfg.Consensus.TimeoutCommit = config.DefaultConsensusConfig().TimeoutCommit
	cfg.Moniker = n.name
	cfg.RPC.ListenAddress = "tcp://0.0.0.0:26657"
	cfg.P2P.PersistentPeers = strings.Join(peers, ",")
	cfg.Instrumentation.Prometheus = true
	cfg.P2P.ListenAddress = "tcp://" + n.publicIP + ":26656"

	// Create a temporary file to write the config
	tmpDir, err := os.MkdirTemp("", "celestia-config-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write config.toml to temp file
	configPath := filepath.Join(tmpDir, "config.toml")
	config.WriteConfigFile(configPath, cfg)

	// Read the config file content
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Write config.toml to remote node
	remoteConfigPath := filepath.Join(n.homeDir, "config", "config.toml")
	if err := n.sshManager.WriteToFile(n.publicIP, remoteConfigPath, string(configContent)); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	// Set correct permissions for config.toml
	if err := n.sshManager.ExecuteCommand(n.publicIP, fmt.Sprintf("chmod 644 %s", remoteConfigPath)); err != nil {
		return fmt.Errorf("failed to set permissions for config file: %w", err)
	}
	fmt.Printf("config.toml written to node %s\n", n.name)

	// Create app.toml
	srvCfg := serverconfig.DefaultConfig()
	srvCfg.BaseConfig.MinGasPrices = fmt.Sprintf("0.001%s", app.BondDenom)
	srvCfg.GRPC.MaxRecvMsgSize = 128 * 1024 * 1024 // 128 MiB
	srvCfg.GRPC.MaxSendMsgSize = 128 * 1024 * 1024 // 128 MiB

	// Validate the configuration
	if err := srvCfg.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid app config: %w", err)
	}

	// Write app.toml to temp file
	appConfigPath := filepath.Join(tmpDir, "app.toml")
	serverconfig.WriteConfigFile(appConfigPath, srvCfg)

	// Read the app config file content
	appConfigContent, err := os.ReadFile(appConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read app config file: %w", err)
	}

	// Write app.toml to remote node
	remoteAppConfigPath := filepath.Join(n.homeDir, "config", "app.toml")
	if err := n.sshManager.WriteToFile(n.publicIP, remoteAppConfigPath, string(appConfigContent)); err != nil {
		return fmt.Errorf("failed to write app config file: %w", err)
	}
	// Set correct permissions for app.toml
	if err := n.sshManager.ExecuteCommand(n.publicIP, fmt.Sprintf("chmod 644 %s", remoteAppConfigPath)); err != nil {
		return fmt.Errorf("failed to set permissions for app config file: %w", err)
	}
	fmt.Printf("app.toml written to node %s\n", n.name)

	return nil
}

// AddressP2P returns the P2P address of the node
func (n *CelestiaNode) AddressP2P() string {
	return fmt.Sprintf("%x@%s:26656", n.networkKey.PublicKey.Address().Bytes(), n.publicIP)
}

// copyNodeKeys copies the node keys to the remote instance
func (n *CelestiaNode) copyNodeKeys(ctx context.Context) error {
	fmt.Printf("Generating keys for node %s...\n", n.name)
	// Create a temporary directory to write the keys
	tmpDir, err := os.MkdirTemp("", "celestia-keys-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config and data directories in temp dir
	for _, dir := range []string{
		filepath.Join(tmpDir, "config"),
		filepath.Join(tmpDir, "data"),
	} {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Set up paths for validator files
	signerKeyPath := filepath.Join(tmpDir, "config", "priv_validator_key.json")
	pvStatePath := filepath.Join(tmpDir, "data", "priv_validator_state.json")

	// Create and save the validator state using privval.NewFilePV
	fmt.Printf("Creating validator state for node %s...\n", n.name)
	filePV := privval.NewFilePV(n.signerKey.PrivateKey, signerKeyPath, pvStatePath)
	filePV.Save()

	// Read the generated files
	signerKeyContent, err := os.ReadFile(signerKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read signer key file: %w", err)
	}

	pvStateContent, err := os.ReadFile(pvStatePath)
	if err != nil {
		return fmt.Errorf("failed to read validator state file: %w", err)
	}

	// Write signer key to remote node
	remoteSignerKeyPath := filepath.Join(n.homeDir, "config", "priv_validator_key.json")
	if err := n.sshManager.WriteToFile(n.publicIP, remoteSignerKeyPath, string(signerKeyContent)); err != nil {
		return fmt.Errorf("failed to write signer key: %w", err)
	}
	// Set correct permissions for priv_validator_key.json
	if err := n.sshManager.ExecuteCommand(n.publicIP, fmt.Sprintf("chmod 600 %s", remoteSignerKeyPath)); err != nil {
		return fmt.Errorf("failed to set permissions for signer key: %w", err)
	}
	fmt.Printf("Validator key written to node %s\n", n.name)

	// Write validator state to remote node
	remotePvStatePath := filepath.Join(n.homeDir, "data", "priv_validator_state.json")
	if err := n.sshManager.WriteToFile(n.publicIP, remotePvStatePath, string(pvStateContent)); err != nil {
		return fmt.Errorf("failed to write validator state file: %w", err)
	}
	fmt.Printf("Validator state written to node %s\n", n.name)

	// Write network key
	remoteNetworkKeyPath := filepath.Join(n.homeDir, "config", "node_key.json")
	localNodeKeyPath := filepath.Join(tmpDir, "config", "node_key.json")
	nodeKey := &p2p.NodeKey{PrivKey: n.networkKey.PrivateKey}
	if err := nodeKey.SaveAs(localNodeKeyPath); err != nil {
		return fmt.Errorf("failed to write network key to temp file: %w", err)
	}

	// Copy node key to remote node
	if err := n.sshManager.CopyFile(n.publicIP, localNodeKeyPath, remoteNetworkKeyPath); err != nil {
		return fmt.Errorf("failed to copy network key: %w", err)
	}
	// Set correct permissions for node_key.json
	if err := n.sshManager.ExecuteCommand(n.publicIP, fmt.Sprintf("chmod 600 %s", remoteNetworkKeyPath)); err != nil {
		return fmt.Errorf("failed to set permissions for node key: %w", err)
	}
	fmt.Printf("Network key written to node %s\n", n.name)

	return nil
}

// GenesisValidator returns the genesis validator configuration for a node
func (n *CelestiaNode) GenesisValidator() genesis.Validator {
	initialTokens := int64(1e16)
	stakeTokens := int64(1e12)
	return genesis.Validator{
		KeyringAccount: genesis.KeyringAccount{
			Name:          n.name,
			InitialTokens: initialTokens,
		},
		ConsensusKey: n.signerKey.PrivateKey,
		NetworkKey:   n.networkKey.PrivateKey,
		Stake:        stakeTokens,
	}
}
