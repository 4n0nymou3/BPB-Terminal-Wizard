#!/bin/bash

BLUE='\033[1;34m'
NC='\033[0m'

echo -e "${BLUE}**********************************************${NC}"
echo -e "${BLUE}*                                            *${NC}"
echo -e "${BLUE}*        BPB Terminal Wizard                 *${NC}"
echo -e "${BLUE}*                                            *${NC}"
echo -e "${BLUE}*  A tool to deploy BPB Panel in terminal    *${NC}"
echo -e "${BLUE}*  Created by Anonymous with thanks to BPB   *${NC}"
echo -e "${BLUE}*                                            *${NC}"
echo -e "${BLUE}**********************************************${NC}"
echo ""

if [ -d "/data/data/com.termux" ] && [ ! -f "/etc/os-release" ]; then
    echo "Detected Termux environment. Setting up Ubuntu..."
    pkg install termux-tools -y
    if ! command -v proot-distro >/dev/null 2>&1; then
        pkg install proot-distro -y
    fi
    if ! proot-distro list | grep -q ubuntu; then
        proot-distro install ubuntu
    fi
    proot-distro login ubuntu -- bash -c "
        apt update
        apt install -y curl wget bash
        curl -fsSL https://deb.nodesource.com/setup_18.x | bash -
        apt install -y nodejs
        npm install -g npm@latest
        npm cache clean --force
        for attempt in {1..3}; do
            npm install -g wrangler@4.12.0 && break
            echo 'Retrying npm install (attempt $attempt)...'
            sleep 5
        done
        mkdir -p /root/.bpb-terminal-wizard
        cd /root/.bpb-terminal-wizard
        curl -L --fail 'https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v1.0/BPB-Terminal-Wizard-linux-arm64' -o BPB-Terminal-Wizard
        chmod +x BPB-Terminal-Wizard
        ./BPB-Terminal-Wizard
    "
else
    if [ -f c "/etc/os-release" ] && grep -q "Ubuntu" /etc/os-release; then
        echo "Detected Ubuntu environment. Setting up dependencies..."
        apt update
        apt install -y curl wget bash
        curl -fsSL https://deb.nodesource.com/setup_18.x | bash -
        apt install -y nodejs
        npm install -g npm@latest
        npm cache clean --force
        for attempt in {1..3}; do
            npm install -g wrangler@4.12.0 && break
            echo "Retrying npm install (attempt $attempt)..."
            sleep 5
        done
        mkdir -p ~/.bpb-terminal-wizard
        cd ~/.bpb-terminal-wizard
        curl -L --fail 'https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v1.0/BPB-Terminal-Wizard-linux-arm64' -o BPB-Terminal-Wizard
        chmod +x BPB-Terminal-Wizard
        ./BPB-Terminal-Wizard
    else
        OS=$(uname -s)
        ARCH=$(uname -m)
        case "$OS" in
          Linux*)  OS_TYPE="linux" ;;
          Darwin*) OS_TYPE="darwin" ;;
          *)       echo "Unsupported OS: $OS"; exit 1 ;;
        esac
        case "$ARCH" in
          x86_64)  ARCH_TYPE="amd64" ;;
          arm64|aarch64) ARCH_TYPE="arm64" ;;
          *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
        esac
        RELEASE_URL="https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v1.0/BPB-Terminal-Wizard-${OS_TYPE}-${ARCH_TYPE}"
        BINARY_NAME="BPB-Terminal-Wizard-${OS_TYPE}-${ARCH_TYPE}"
        echo "Downloading $BINARY_NAME..."
        curl -L --fail "$RELEASE_URL" -o "$BINARY_NAME" || { echo "Error downloading $BINARY_NAME"; exit 1; }
        chmod +x "$BINARY_NAME"
        mkdir -p ~/.bpb-terminal-wizard
        mv "$BINARY_NAME" ~/.bpb-terminal-wizard/BPB-Terminal-Wizard
        cd ~/.bpb-terminal-wizard
        echo "Running BPB Terminal Wizard..."
        ./BPB-Terminal-Wizard
    fi
fi
