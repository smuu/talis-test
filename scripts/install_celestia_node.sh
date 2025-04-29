#!/bin/bash

# Exit on error
set -e

echo "Starting Celestia Node installation..."

# Create a temporary script that will automatically answer the prompt
cat > /tmp/auto_install_node.sh << 'EOF'
#!/bin/bash
# Download the celestia-node installation script
curl -sL https://docs.celestia.org/celestia-node.sh > /tmp/celestia-node.sh

# Check if version parameter is provided
if [ -z "$1" ]; then
    # Run the script and automatically select option 2 (system bin directory)
    echo "2" | bash /tmp/celestia-node.sh
else
    # Run the script with version parameter and option 2
    echo "2" | bash /tmp/celestia-node.sh -v "$1"
fi
EOF

# Make the script executable
chmod +x /tmp/auto_install_node.sh

# Check if version parameter is provided
if [ -z "$1" ]; then
    echo "Installing latest version of Celestia Node..."
    /tmp/auto_install_node.sh
else
    echo "Installing Celestia Node version $1..."
    /tmp/auto_install_node.sh "$1"
fi

# Verify installation
echo "Verifying installation..."
# Check in system bin directory
if [ -f "/usr/local/bin/celestia" ]; then
    /usr/local/bin/celestia version > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Celestia Node installed successfully in /usr/local/bin!"
        exit 0
    fi
fi

# Check in Go bin directory
if [ -d "$HOME/go/bin" ] && [ -f "$HOME/go/bin/celestia" ]; then
    $HOME/go/bin/celestia version > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Celestia Node installed successfully in $HOME/go/bin!"
        # Ensure it's in PATH
        if ! grep -q "export PATH=\$PATH:\$HOME/go/bin" ~/.bashrc; then
            echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc
        fi
        exit 0
    fi
fi

# Check in temp directory
if [ -d "/root/celestia-node-temp" ] && [ -f "/root/celestia-node-temp/celestia" ]; then
    /root/celestia-node-temp/celestia version > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Celestia Node installed successfully in /root/celestia-node-temp!"
        # Add to PATH for future use
        if ! grep -q "export PATH=\$PATH:/root/celestia-node-temp" ~/.bashrc; then
            echo 'export PATH=$PATH:/root/celestia-node-temp' >> ~/.bashrc
        fi
        exit 0
    fi
fi

echo "Failed to verify Celestia Node installation"
exit 1
