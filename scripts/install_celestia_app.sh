#!/bin/bash

# Exit on error
set -e

echo "Starting Celestia App installation..."

# Configure BBR
echo "Configuring system to use BBR..."
if [ "$(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}')" != "bbr" ]; then
    echo "BBR is not enabled. Configuring BBR..."
    sudo modprobe tcp_bbr && \
    echo tcp_bbr | sudo tee -a /etc/modules && \
    echo "net.core.default_qdisc=fq" | sudo tee -a /etc/sysctl.conf && \
    echo "net.ipv4.tcp_congestion_control=bbr" | sudo tee -a /etc/sysctl.conf && \
    sudo sysctl -p && \
    echo "BBR has been enabled." || \
    echo "Failed to enable BBR. Please check error messages above."
else
    echo "BBR is already enabled."
fi

# Create a temporary script that will automatically answer the prompt
cat > /tmp/auto_install.sh << 'EOF'
#!/bin/bash
# Download the celestia-app installation script
curl -sL https://docs.celestia.org/celestia-app.sh > /tmp/celestia-app.sh

# Check if version parameter is provided
if [ -z "$1" ]; then
    # Run the script with option 2 (system bin directory)
    echo "2" | bash /tmp/celestia-app.sh
else
    # Run the script with version parameter and option 2
    echo "2" | bash /tmp/celestia-app.sh -v "$1"
fi
EOF

# Make the script executable
chmod +x /tmp/auto_install.sh

# Check if version parameter is provided
if [ -z "$1" ]; then
    echo "Installing latest version of Celestia App..."
    /tmp/auto_install.sh
else
    echo "Installing Celestia App version $1..."
    /tmp/auto_install.sh "$1"
fi

# Verify installation
echo "Verifying installation..."
# Check in system bin directory
if [ -f "/usr/local/bin/celestia-appd" ]; then
    /usr/local/bin/celestia-appd version > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Celestia App installed successfully in /usr/local/bin!"
        exit 0
    fi
fi

# Check in Go bin directory
if [ -d "$HOME/go/bin" ] && [ -f "$HOME/go/bin/celestia-appd" ]; then
    $HOME/go/bin/celestia-appd version > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Celestia App installed successfully in $HOME/go/bin!"
        # Ensure it's in PATH
        if ! grep -q "export PATH=\$PATH:\$HOME/go/bin" ~/.bashrc; then
            echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc
        fi
        exit 0
    fi
fi

# Check in temp directory
if [ -d "/root/celestia-app-temp" ] && [ -f "/root/celestia-app-temp/celestia-appd" ]; then
    /root/celestia-app-temp/celestia-appd version > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Celestia App installed successfully in /root/celestia-app-temp!"
        # Add to PATH for future use
        if ! grep -q "export PATH=\$PATH:/root/celestia-app-temp" ~/.bashrc; then
            echo 'export PATH=$PATH:/root/celestia-app-temp' >> ~/.bashrc
        fi
        exit 0
    fi
fi

echo "Failed to verify Celestia App installation"
exit 1
