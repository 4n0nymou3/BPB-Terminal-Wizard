#!/bin/bash

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
echo -e "${BOLD_BLUE}║ ${RESET}${BOLD}BPB Terminal Wizard${RESET}"
echo -e "${BOLD_BLUE}║ ${RESET}"
echo -e "${BOLD_BLUE}║ ${CYAN}A tool to deploy BPB Panel easily${RESET}"
echo -e "${BOLD_BLUE}║ ${CYAN}Created by ${RESET}${BOLD_GREEN}Anonymous${RESET}${CYAN} with thanks to ${RESET}${BOLD_GREEN}BPB${RESET}"
echo -e "${BOLD_BLUE}╚═══════════════════════════════════════════${RESET}"
echo ""

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
        apt install -y curl wget bash nodejs npm git
        if ! command -v node >/dev/null 2>&1 || ! node -v | grep -qE 'v18.|v20.|v22.'; then
            echo -e '${YELLOW}ℹ Node.js not found or version is too old. Installing/Updating Node.js 18...${RESET}'
            curl -fsSL https://deb.nodesource.com/setup_18.x | bash -
            apt install -y nodejs
        fi
        echo -e '${BLUE}❯ Updating npm...${RESET}'
        npm install -g npm@latest
        echo -e '${BLUE}❯ Cleaning npm cache...${RESET}'
        npm cache clean --force
        echo -e '${BLUE}❯ Installing Wrangler v4.12.0...${RESET}'
        for attempt in {1..3}; do
            echo -e '${YELLOW}ℹ Attempt \$attempt to install Wrangler...${RESET}'
            if npm install -g wrangler@4.12.0; then
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
        echo -e '${BLUE}❯ Preparing BPB Terminal Wizard directory...${RESET}'
        mkdir -p /root/.bpb-terminal-wizard
        cd /root/.bpb-terminal-wizard
        echo -e '${BLUE}❯ Downloading BPB Terminal Wizard...${RESET}'
        curl -L --fail 'https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v1.1/BPB-Terminal-Wizard-linux-arm64' -o BPB-Terminal-Wizard || { echo -e '${RED}✗ Error downloading BPB Terminal Wizard${RESET}'; exit 1; }
        chmod +x BPB-Terminal-Wizard
        echo -e '${BLUE}❯ Running BPB Terminal Wizard...${RESET}'
        ./BPB-Terminal-Wizard
    "
else
    OS=\$(uname -s)
    ARCH=\$(uname -m)
    OS_TYPE=""
    ARCH_TYPE=""

    case "\$OS" in
      Linux*)  OS_TYPE="linux" ;;
      Darwin*) OS_TYPE="darwin" ;;
      *)       echo -e "${RED}✗ Unsupported OS: \$OS${RESET}"; exit 1 ;;
    esac
    case "\$ARCH" in
      x86_64)  ARCH_TYPE="amd64" ;;
      arm64|aarch64) ARCH_TYPE="arm64" ;;
      *)       echo -e "${RED}✗ Unsupported architecture: \$ARCH${RESET}"; exit 1 ;;
    esac

    if [ "\$OS_TYPE" == "linux" ] && [ -f "/etc/os-release" ] && grep -q "Ubuntu" /etc/os-release; then
        echo -e "${YELLOW}ℹ Detected Ubuntu environment. Setting up dependencies...${RESET}"
        apt update && apt upgrade -y
        apt install -y curl wget bash nodejs npm git
         if ! command -v node >/dev/null 2>&1 || ! node -v | grep -qE 'v18.|v20.|v22.'; then
            echo -e '${YELLOW}ℹ Node.js not found or version is too old. Installing/Updating Node.js 18...${RESET}'
            curl -fsSL https://deb.nodesource.com/setup_18.x | bash -
            apt install -y nodejs
        fi
        echo -e '${BLUE}❯ Updating npm...${RESET}'
        npm install -g npm@latest
        echo -e '${BLUE}❯ Cleaning npm cache...${RESET}'
        npm cache clean --force
        echo -e '${BLUE}❯ Installing Wrangler v4.12.0...${RESET}'
         for attempt in {1..3}; do
            echo -e "${YELLOW}ℹ Attempt \$attempt to install Wrangler...${RESET}"
            if npm install -g wrangler@4.12.0; then
                 echo -e "${GREEN}✓ Wrangler installed successfully.${RESET}"
                 break
            fi
            if [ \$attempt -eq 3 ]; then
                 echo -e "${RED}✗ Failed to install Wrangler after 3 attempts.${RESET}"
                 exit 1
            fi
            echo -e "${YELLOW}ℹ Retrying npm install in 5 seconds...${RESET}"
            sleep 5
        done
    elif ! command -v node >/dev/null 2>&1 || ! node -v | grep -qE 'v18.|v20.|v22.'; then
         echo -e "${RED}✗ Node.js v18+ is required. Please install it first.${RESET}"
         exit 1
    elif ! command -v npm >/dev/null 2>&1; then
         echo -e "${RED}✗ npm is required. Please install it first.${RESET}"
         exit 1
    elif ! command -v git >/dev/null 2>&1; then
         echo -e "${RED}✗ git is required. Please install it first.${RESET}"
         exit 1
    fi

    INSTALL_DIR="\$HOME/.bpb-terminal-wizard"
    BINARY_NAME="BPB-Terminal-Wizard"
    RELEASE_URL="https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v1.1/BPB-Terminal-Wizard-\${OS_TYPE}-\${ARCH_TYPE}"

    echo -e "${BLUE}❯ Preparing BPB Terminal Wizard directory...${RESET}"
    mkdir -p "\$INSTALL_DIR"
    cd "\$INSTALL_DIR" || { echo -e "${RED}✗ Could not change to directory \$INSTALL_DIR${RESET}"; exit 1; }

    echo -e "${BLUE}❯ Downloading \$BINARY_NAME for \${OS_TYPE}-\${ARCH_TYPE}...${RESET}"
    curl -L --fail "\$RELEASE_URL" -o "\$BINARY_NAME" || { echo -e "${RED}✗ Error downloading \$BINARY_NAME${RESET}"; exit 1; }
    chmod +x "\$BINARY_NAME"

    echo -e "${BLUE}❯ Running BPB Terminal Wizard...${RESET}"
    ./"\$BINARY_NAME"
fi

echo -e "${GREEN}✓ Installation script finished.${RESET}"