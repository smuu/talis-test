#!/bin/bash

# Exit on error
set -e

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check if jq is installed
if ! command_exists jq; then
    echo "jq is not installed. Installing..."
    sudo apt-get update
    sudo apt-get install -y jq
fi

# Get the current user
USER=$(whoami)

# Check if service already exists
if systemctl is-active --quiet celestia-appd; then
    echo "Celestia App service is already running. Restarting..."
    sudo systemctl restart celestia-appd
else
    # Create the systemd service file
    echo "Creating Celestia App systemd service file..."
    sudo tee /etc/systemd/system/celestia-appd.service > /dev/null << EOF
[Unit]
Description=celestia-appd Cosmos daemon
After=network-online.target

[Service]
User=$USER
ExecStart=/usr/local/bin/celestia-appd start
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

    # Verify the service file was created
    echo "Verifying service file content..."
    cat /etc/systemd/system/celestia-appd.service

    # Reload systemd to recognize the new service
    echo "Reloading systemd..."
    sudo systemctl daemon-reload

    # Enable and start the service
    echo "Enabling and starting celestia-appd service..."
    sudo systemctl enable celestia-appd
    sudo systemctl start celestia-appd
fi

# Check service status
echo "Checking service status..."
sudo systemctl status celestia-appd

echo "Celestia App daemon setup completed successfully!" 
