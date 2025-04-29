#!/bin/bash

VERSION="1.3"

RESET='\033[0m'
BOLD='\033[1m'
BLUE='\033[0;34m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
BOLD_BLUE='\033[1;34m'
BOLD_GREEN='\033[1;32m'

echo -e "${BOLD_BLUE}╔═══════════════════════════════════════════${RESET}"
echo -e "${BOLD_BLUE}║ ${RESET}${BOLD}BPB Terminal Wizard v${VERSION}${RESET}"
echo -e "${BOLD_BLUE}║ ${RESET}"
echo -e "${BOLD_BLUE}║ ${CYAN}A tool to deploy BPB Panel easily${RESET}"
echo -e "${BOLD_BLUE}║ ${CYAN}Created by ${RESET}${BOLD_GREEN}Anonymous${RESET}${CYAN} with thanks to ${RESET}${BOLD_GREEN}BPB${RESET}"
echo -e "${BOLD_BLUE}╚═══════════════════════════════════════════${RESET}"
echo ""

check_node_version() {
    if command -v node >/dev/null 2>&1; then
        NODE_VERSION=$(node -v | cut -d'v' -f2)
        NODE_MAJOR=$(echo "$NODE_VERSION" | cut -d'.' -f1)
        if [ "$NODE_MAJOR" -lt 18 ]; then
            echo -e "${YELLOW}ℹ Node.js version $NODE_VERSION is too old. Installing latest Node.js...${RESET}"
            return 1
        else
            echo -e "${GREEN}✓ Node.js version $NODE_VERSION is compatible.${RESET}"
            return 0
        fi
    else
        echo -e "${YELLOW}ℹ Node.js not found. Installing latest Node.js...${RESET}"
        return 1
    fi
}

check_npm_version() {
    if command -v npm >/dev/null 2>&1; then
        NPM_VERSION=$(npm -v)
        LATEST_NPM=$(npm view npm version)
        if [ "$NPM_VERSION" != "$LATEST_NPM" ]; then
            echo -e "${YELLOW}ℹ npm version $NPM_VERSION is outdated. Installing latest npm...${RESET}"
            return 1
        else
            echo -e "${GREEN}✓ npm version $NPM_VERSION is up-to-date.${RESET}"
            return 0
        fi
    else
        echo -e "${YELLOW}ℹ npm not found. Installing latest npm...${RESET}"
        return 1
    fi
}

check_wrangler_version() {
    if command -v wrangler >/dev/null 2>&1; then
        WRANGLER_VERSION=$(npx wrangler --version)
        LATEST_WRANGLER=$(npm view wrangler version)
        if [ "$WRANGLER_VERSION" != "$LATEST_WRANGLER" ]; then
            echo -e "${YELLOW}ℹ Wrangler version $WRANGLER_VERSION is outdated. Installing latest Wrangler...${RESET}"
            return 1
        else
            echo -e "${GREEN}✓ Wrangler version $WRANGLER_VERSION is up-to-date.${RESET}"
            return 0
        fi
    else
        echo -e "${YELLOW}ℹ Wrangler not found. Installing latest Wrangler...${RESET}"
        return 1
    fi
}

if [ -d "/data/data/com.termux" ] && [ ! -f "/etc/os-release" ]; then
    echo -e "${YELLOW}ℹ Detected Termux environment. Setting up Ubuntu...${RESET}"
    pkg update -y && pkg upgrade -y
    pkg install termux-tools proot-distro wget curl -y
    if ! proot-distro list | grep -q ubuntu; then
        echo -e "${BLUE}❯ Installing Ubuntu distribution...${RESET}"
        proot-distro install ubuntu
    else
        echo -e "${GREEN}✓ Ubuntu distribution already installed.${RESET}"
    fi
    echo -e "${BLUE}❯ Logging into Ubuntu and setting up dependencies...${RESET}"
    proot-distro login ubuntu -- bash -c "
        apt update && apt upgrade -y
        apt install -y curl wget bash npm git
        if ! check_node_version; then
            curl -fsSL https://deb.nodesource.com/setup_current.x | bash -
            apt install -y nodejs
        fi
        if ! check_npm_version; then
            npm install -g npm@latest
        fi
        echo -e '${BLUE}❯ Cleaning npm cache...${RESET}'
        npm cache clean --force
        if ! check_wrangler_version; then
            echo -e '${BLUE}❯ Installing Wrangler...${RESET}'
            for attempt in {1..3}; do
                echo -e '${YELLOW}ℹ Attempt $attempt to install Wrangler...${RESET}'
                if npm install -g wrangler; then
                    echo -e '${GREEN}✓ Wrangler installed successfully.${RESET}'
                    break
                fi
                if [ \$attempt -eq 3 ]; then
                    echo -e '${RED}✗ Failed to install Wrangler after 3 attempts.${RESET}'
                    exit 1
                fi
                echo -e '${YELLOW}ℹ Retrying npm install in 5 seconds...${RESET}'
                sleep 5
            done
        fi
        echo -e '${BLUE}❯ Preparing BPB Terminal Wizard directory...${RESET}'
        mkdir -p /root/.bpb-terminal-wizard
        cd /root/.bpb-terminal-wizard
        echo -e '${BLUE}❯ Downloading BPB Terminal Wizard...${RESET}'
        for attempt in {1..3}; do
            if curl -L --fail 'https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v${VERSION}/BPB-Terminal-Wizard-linux-arm64' -o BPB-Terminal-Wizard; then
                echo -e '${GREEN}✓ Download successful.${RESET}'
                break
            fi
            if [ \$attempt -eq 3 ]; then
                echo -e '${RED}✗ Failed to download BPB Terminal Wizard after 3 attempts.${RESET}'
                exit 1
            fi
            echo -e '${YELLOW}ℹ Retrying download in 5 seconds...${RESET}'
            sleep 5
        done
        chmod +x BPB-Terminal-Wizard
        echo -e '${BLUE}❯ Running BPB Terminal Wizard...${RESET}'
        ./BPB-Terminal-Wizard
    "
elif [ "$(uname -s)" == "Darwin" ]; then
    echo -e "${YELLOW}ℹ Detected macOS environment. Setting up dependencies...${RESET}"
    if ! command -v brew >/dev/null 2>&1; then
        echo -e "${RED}✗ Homebrew is not installed. Please install Homebrew first.${RESET}"
        exit 1
    fi
    brew update
    if ! check_node_version; then
        brew install node
    fi
    brew install git
    if ! check_npm_version; then
        npm install -g npm@latest
    fi
    echo -e "${BLUE}❯ Cleaning npm cache...${RESET}"
    npm cache clean --force
    if ! check_wrangler_version; then
        echo -e "${BLUE}❯ Installing Wrangler...${RESET}"
        for attempt in {1..3}; do
            echo -e "${YELLOW}ℹ Attempt $attempt to install Wrangler...${RESET}"
            if npm install -g wrangler; then
                echo -e "${GREEN}✓ Wrangler installed successfully.${RESET}"
                break
            fi
            if [ $attempt -eq 3 ]; then
                echo -e "${RED}✗ Failed to install Wrangler after 3 attempts.${RESET}"
                exit 1
            fi
            echo -e "${YELLOW}ℹ Retrying npm install in 5 seconds...${RESET}"
            sleep 5
        done
    fi
    INSTALL_DIR="$HOME/.bpb-terminal-wizard"
    BINARY_NAME="BPB-Terminal-Wizard"
    ARCH_TYPE=$(uname -m)
    if [ "$ARCH_TYPE" == "x86_64" ]; then
        ARCH_TYPE="amd64"
    elif [ "$ARCH_TYPE" == "arm64" ]; then
        ARCH_TYPE="arm64"
    else
        echo -e "${RED}✗ Unsupported architecture: $ARCH_TYPE${RESET}"
        exit 1
    fi
    RELEASE_URL="https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v${VERSION}/BPB-Terminal-Wizard-darwin-${ARCH_TYPE}"
    echo -e "${BLUE}❯ Preparing BPB Terminal Wizard directory...${RESET}"
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR" || { echo -e "${RED}✗ Could not change to directory $INSTALL_DIR${RESET}"; exit 1; }
    echo -e "${BLUE}❯ Downloading $BINARY_NAME for darwin-${ARCH_TYPE}...${RESET}"
    for attempt in {1..3}; do
        if curl -L --fail "$RELEASE_URL" -o "$BINARY_NAME"; then
            echo -e "${GREEN}✓ Download successful.${RESET}"
            break
        fi
        if [ $attempt -eq 3 ]; then
            echo -e "${RED}✗ Failed to download $BINARY_NAME after 3 attempts.${RESET}"
            exit 1
        fi
        echo -e "${YELLOW}ℹ Retrying download in 5 seconds...${RESET}"
        sleep 5
    done
    chmod +x "$BINARY_NAME"
    echo -e "${BLUE}❯ Running BPB Terminal Wizard...${RESET}"
    ./"$BINARY_NAME"
else
    OS=$(uname -s)
    ARCH=$(uname -m)
    OS_TYPE=""
    ARCH_TYPE=""
    case "$OS" in
      Linux*)  OS_TYPE="linux" ;;
      *)       echo -e "${RED}✗ Unsupported OS: $OS${RESET}"; exit 1 ;;
    esac
    case "$ARCH" in
      x86_64)  ARCH_TYPE="amd64" ;;
      arm64|aarch64) ARCH_TYPE="arm64" ;;
      *)       echo -e "${RED}✗ Unsupported architecture: $ARCH${RESET}"; exit 1 ;;
    esac
    if [ "$OS_TYPE" == "linux" ] && [ -f "/etc/os-release" ] && grep -q "Ubuntu" /etc/os-release; then
        echo -e "${YELLOW}ℹ Detected Ubuntu environment. Setting up dependencies...${RESET}"
        apt update && apt upgrade -y
        apt install -y curl wget bash npm git
        if ! check_node_version; then
            curl -fsSL https://deb.nodesource.com/setup_current.x | bash -
            apt install -y nodejs
        fi
        if ! check_npm_version; then
            npm install -g npm@latest
        fi
        echo -e "${BLUE}❯ Cleaning npm cache...${RESET}"
        npm cache clean --force
        if ! check_wrangler_version; then
            echo -e "${BLUE}❯ Installing Wrangler...${RESET}"
            for attempt in {1..3}; do
                echo -e "${YELLOW}ℹ Attempt $attempt to install Wrangler...${RESET}"
                if npm install -g wrangler; then
                    echo -e "${GREEN}✓ Wrangler installed successfully.${RESET}"
                    break
                fi
                if [ $attempt -eq 3 ]; then
                    echo -e "${RED}✗ Failed to install Wrangler after 3 attempts.${RESET}"
                    exit 1
                fi
                echo -e "${YELLOW}ℹ Retrying npm install in 5 seconds...${RESET}"
                sleep 5
            done
        fi
    else
        echo -e "${RED}✗ This script only supports Ubuntu on Linux. Please install dependencies manually.${RESET}"
        exit 1
    fi
    INSTALL_DIR="$HOME/.bpb-terminal-wizard"
    BINARY_NAME="BPB-Terminal-Wizard"
    RELEASE_URL="https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v${VERSION}/BPB-Terminal-Wizard-${OS_TYPE}-${ARCH_TYPE}"
    echo -e "${BLUE}❯ Preparing BPB Terminal Wizard directory...${RESET}"
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR" || { echo -e "${RED}✗ Could not change to directory $INSTALL_DIR${RESET}"; exit 1; }
    echo -e "${BLUE}❯ Downloading $BINARY_NAME for ${OS_TYPE}-${ARCH_TYPE}...${RESET}"
    for attempt in {1..3}; do
        if curl -L --fail "$RELEASE_URL" -o "$BINARY_NAME"; then
            echo -e "${GREEN}✓ Download successful.${RESET}"
            break
        fi
        if [ $attempt -eq 3 ]; then
            echo -e "${RED}✗ Failed to download $BINARY_NAME after 3 attempts.${RESET}"
            exit 1
        fi
        echo -e "${YELLOW}ℹ Retrying download in 5 seconds...${RESET}"
        sleep 5
    done
    chmod +x "$BINARY_NAME"
    echo -e "${BLUE}❯ Running BPB Terminal Wizard...${RESET}"
    ./"$BINARY_NAME"
fi

echo -e "${GREEN}✓ Installation script finished.${RESET}"