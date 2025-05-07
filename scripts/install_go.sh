#!/bin/bash

# Exit on error
set -e

echo "Starting Go installation..."

# Install required packages
echo "Installing required packages..."

# Check if apt is locked and wait until it's available
while sudo lsof /var/lib/apt/lists/lock >/dev/null 2>&1 || sudo lsof /var/lib/dpkg/lock-frontend >/dev/null 2>&1 || sudo lsof /var/lib/dpkg/lock >/dev/null 2>&1; do
    echo "Waiting for apt locks to be released..."
    sleep 5
done

sudo apt update
sudo apt install -y curl tar wget aria2 clang pkg-config libssl-dev jq build-essential git make ncdu

# Install Go
echo "Installing Go..."
ver="$1"
wget "https://golang.org/dl/go$ver.linux-amd64.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "go$ver.linux-amd64.tar.gz"
rm "go$ver.linux-amd64.tar.gz"

# Add PATH to shell profile and current session
echo "Setting up PATH..."
PATH_LINE='export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin'

# Add to .bashrc if it exists, otherwise to .bash_profile
if [ -f "$HOME/.bashrc" ]; then
    echo "Adding Go to .bashrc..."
    echo "$PATH_LINE" >> "$HOME/.bashrc"
    source "$HOME/.bashrc"
elif [ -f "$HOME/.bash_profile" ]; then
    echo "Adding Go to .bash_profile..."
    echo "$PATH_LINE" >> "$HOME/.bash_profile"
    source "$HOME/.bash_profile"
else
    echo "Creating and using .bashrc..."
    echo "$PATH_LINE" >> "$HOME/.bashrc"
    source "$HOME/.bashrc"
fi

# Also export for current session
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin

# Verify installation
echo "Verifying Go installation..."
go version

echo "Go installation completed successfully!" 
