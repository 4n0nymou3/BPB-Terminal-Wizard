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

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

install_nodejs() {
    echo -e "${YELLOW}ℹ Node.js not found or version is too old. Installing/Updating Node.js 18...${RESET}"
    if command_exists curl; then
        curl -fsSL https://deb.nodesource.com/setup_18.x | bash -
    elif command_exists wget; then
        wget -qO- https://deb.nodesource.com/setup_18.x | bash -
    else
        echo -e "${RED}✗ Neither curl nor wget found. Please install one to download Node.js setup script.${RESET}"
        return 1
    fi
    if ! apt install -y nodejs; then
        echo -e "${RED}✗ Failed to install nodejs.${RESET}"
        return 1
    fi
    return 0
}

if [ -d "/data/data/com.termux" ] && [ ! -f "/etc/os-release" ]; then
    echo -e "${YELLOW}ℹ Detected Termux environment. Setting up Ubuntu...${RESET}"
    pkg update -y && pkg upgrade -y || echo -e "${YELLOW}⚠ Failed to update/upgrade Termux packages, continuing...${RESET}"
    pkg install termux-tools proot-distro wget curl -y || { echo -e "${RED}✗ Failed to install Termux dependencies.${RESET}"; exit 1; }

    if ! proot-distro list | grep -q ubuntu; then
        echo -e "${BLUE}❯ Installing Ubuntu distribution...${RESET}"
        proot-distro install ubuntu || { echo -e "${RED}✗ Failed to install Ubuntu distribution.${RESET}"; exit 1; }
    else
         echo -e "${GREEN}✓ Ubuntu distribution already installed.${RESET}"
    fi

    echo -e "${BLUE}❯ Logging into Ubuntu and setting up dependencies...${RESET}"
    proot-distro login ubuntu -- bash -c "
        apt update && apt upgrade -y || echo -e '${YELLOW}⚠ Failed to update/upgrade Ubuntu packages, continuing...${RESET}'
        apt install -y curl wget bash nodejs npm git || { echo -e '${RED}✗ Failed to install Ubuntu dependencies.${RESET}'; exit 1; }

        if ! command -v node >/dev/null 2>&1 || ! node -v | grep -qE 'v18.|v20.|v22.'; then
            if ! install_nodejs; then
                exit 1
            fi
        fi

        echo -e '${BLUE}❯ Updating npm...${RESET}'
        npm install -g npm@latest || echo -e '${YELLOW}⚠ Failed to update npm, continuing...${RESET}'

        echo -e '${BLUE}❯ Cleaning npm cache...${RESET}'
        npm cache clean --force || echo -e '${YELLOW}⚠ Failed to clean npm cache, continuing...${RESET}'

        echo -e '${BLUE}❯ Installing Wrangler v4.12.0...${RESET}'
        WRANGLER_INSTALLED=0
        for attempt in {1..3}; do
            echo -e '${YELLOW}ℹ Attempt \$attempt to install Wrangler...${RESET}'
            if npm install -g wrangler@4.12.0; then
                echo -e '${GREEN}✓ Wrangler installed successfully.${RESET}'
                WRANGLER_INSTALLED=1
                break
            fi
            if [ \$attempt -eq 3 ]; then
                 echo -e '${RED}✗ Failed to install Wrangler after 3 attempts.${RESET}'
                 exit 1
            fi
            echo -e '${YELLOW}ℹ Retrying npm install in 5 seconds...${RESET}'
            sleep 5
        done

        if [ \$WRANGLER_INSTALLED -eq 1 ]; then
            WRANGLER_VERSION=\$(npx wrangler --version 2>/dev/null)
            if [[ "\$WRANGLER_VERSION" != *"4.12.0"* ]]; then
                echo -e '${RED}✗ Installed Wrangler version \$WRANGLER_VERSION does not match required version 4.12.0.${RESET}'
                exit 1
            else
                echo -e '${GREEN}✓ Wrangler version 4.12.0 verified.${RESET}'
            fi
        else
             echo -e '${RED}✗ Wrangler was not installed successfully.${RESET}'
             exit 1
        fi


        echo -e '${BLUE}❯ Preparing BPB Terminal Wizard directory...${RESET}'
        mkdir -p /root/.bpb-terminal-wizard || { echo -e '${RED}✗ Could not create directory /root/.bpb-terminal-wizard${RESET}'; exit 1; }
        cd /root/.bpb-terminal-wizard || { echo -e '${RED}✗ Could not change to directory /root/.bpb-terminal-wizard${RESET}'; exit 1; }

        echo -e '${BLUE}❯ Downloading BPB Terminal Wizard...${RESET}'
        curl -L --fail 'https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v1.1/BPB-Terminal-Wizard-linux-arm64' -o BPB-Terminal-Wizard || { echo -e '${RED}✗ Error downloading BPB Terminal Wizard${RESET}'; exit 1; }
        chmod +x BPB-Terminal-Wizard || { echo -e '${RED}✗ Error making BPB-Terminal-Wizard executable${RESET}'; exit 1; }

        echo -e '${BLUE}❯ Running BPB Terminal Wizard...${RESET}'
        ./BPB-Terminal-Wizard || { echo -e '${RED}✗ Error running BPB-Terminal-Wizard${RESET}'; exit 1; }
    "
else
    OS=$(uname -s)
    ARCH=$(uname -m)
    OS_TYPE=""
    ARCH_TYPE=""

    case "$OS" in
      Linux*)  OS_TYPE="linux" ;;
      Darwin*) OS_TYPE="darwin" ;;
      *)       echo -e "${RED}✗ Unsupported OS: $OS${RESET}"; exit 1 ;;
    esac

    case "$ARCH" in
      x86_64)  ARCH_TYPE="amd64" ;;
      arm64|aarch64) ARCH_TYPE="arm64" ;;
      *)       echo -e "${RED}✗ Unsupported architecture: $ARCH${RESET}"; exit 1 ;;
    esac

    if ! command_exists curl && ! command_exists wget; then
        echo -e "${RED}✗ Neither curl nor wget found. Please install one.${RESET}"
        exit 1
    fi
    if ! command_exists bash; then
        echo -e "${RED}✗ bash is required. Please install it.${RESET}"
        exit 1
     fi
     if ! command_exists git; then
        echo -e "${RED}✗ git is required. Please install it first.${RESET}"
        exit 1
    fi


    if [ "$OS_TYPE" == "linux" ] && [ -f "/etc/os-release" ]; then
        if grep -q "Ubuntu" /etc/os-release || grep -q "Debian" /etc/os-release; then
            echo -e "${YELLOW}ℹ Detected Ubuntu/Debian environment. Setting up dependencies...${RESET}"
            apt update && apt upgrade -y || echo -e "${YELLOW}⚠ Failed to update/upgrade apt packages, continuing...${RESET}"
            apt install -y curl wget bash nodejs npm git || { echo -e "${RED}✗ Failed to install apt dependencies.${RESET}"; exit 1; }
        elif grep -q "Fedora" /etc/os-release || grep -q "CentOS" /etc/os-release || grep -q "RHEL" /etc/os-release; then
             echo -e "${YELLOW}ℹ Detected Fedora/CentOS/RHEL environment. Setting up dependencies...${RESET}"
             sudo dnf check-update || echo -e "${YELLOW}⚠ Failed to check for dnf updates, continuing...${RESET}"
             sudo dnf -y upgrade || echo -e "${YELLOW}⚠ Failed to upgrade dnf packages, continuing...${RESET}"
             sudo dnf install -y curl wget bash nodejs npm git || { echo -e "${RED}✗ Failed to install dnf dependencies.${RESET}'; exit 1; }"
        elif grep -q "Alpine" /etc/os-release; then
             echo -e "${YELLOW}ℹ Detected Alpine environment. Setting up dependencies...${RESET}"
             apk update && apk upgrade || echo -e "${YELLOW}⚠ Failed to update/upgrade apk packages, continuing...${RESET}"
             apk add --no-cache curl wget bash nodejs npm git || { echo -e "${RED}✗ Failed to install apk dependencies.${RESET}'; exit 1; }"
        else
            echo -e "${YELLOW}ℹ Detected other Linux environment. Attempting to install dependencies using common package managers.${RESET}"
            if command_exists apt; then
                 apt update && apt upgrade -y || echo -e "${YELLOW}⚠ Failed to update/upgrade apt packages, continuing...${RESET}"
                 apt install -y curl wget bash nodejs npm git || { echo -e "${RED}✗ Failed to install apt dependencies.${RESET}"; exit 1; }
            elif command_exists dnf; then
                 sudo dnf check-update || echo -e "${YELLOW}⚠ Failed to check for dnf updates, continuing...${RESET}"
                 sudo dnf -y upgrade || echo -e "${YELLOW}⚠ Failed to upgrade dnf packages, continuing...${RESET}"
                 sudo dnf install -y curl wget bash nodejs npm git || { echo -e "${RED}✗ Failed to install dnf dependencies.${RESET}'; exit 1; }"
            elif command_exists pacman; then
                 sudo pacman -Sy --noconfirm || echo -e "${YELLOW}⚠ Failed to sync pacman databases, continuing...${RESET}"
                 sudo pacman -Su --noconfirm || echo -e "${YELLOW}⚠ Failed to upgrade pacman packages, continuing...${RESET}"
                 sudo pacman -S --noconfirm curl wget bash nodejs npm git || { echo -e "${RED}✗ Failed to install pacman dependencies.${RESET}'; exit 1; }"
            elif command_exists zypper; then
                 sudo zypper refresh || echo -e "${YELLOW}⚠ Failed to refresh zypper repositories, continuing...${RESET}"
                 sudo zypper update -y || echo -e "${YELLOW}⚠ Failed to update zypper packages, continuing...${RESET}"
                 sudo zypper install -y curl wget bash nodejs npm git || { echo -e "${RED}✗ Failed to install zypper dependencies.${RESET}'; exit 1; }"
            else
                 echo -e "${RED}✗ No supported package manager found (apt, dnf, pacman, zypper). Please install dependencies manually.${RESET}"
                 exit 1
            fi
        fi
    elif [ "$OS_TYPE" == "darwin" ]; then
        echo -e "${YELLOW}ℹ Detected macOS environment. Setting up dependencies...${RESET}"
        if ! command_exists brew; then
            echo -e "${YELLOW}ℹ Homebrew not found. Installing Homebrew...${RESET}"
            /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" || { echo -e "${RED}✗ Failed to install Homebrew.${RESET}"; exit 1; }
            export PATH="/opt/homebrew/bin:$PATH"
        fi
        brew update && brew upgrade || echo -e "${YELLOW}⚠ Failed to update/upgrade Homebrew packages, continuing...${RESET}"
        brew install curl wget bash node@18 git || { echo -e "${RED}✗ Failed to install Homebrew dependencies.${RESET}"; exit 1; }
    fi


    if ! command -v node >/dev/null 2>&1 || ! node -v | grep -qE 'v18.|v20.|v22.'; then
         echo -e "${RED}✗ Node.js v18+ is required. Please install it first.${RESET}"
         exit 1
    fi

    if ! command -v npm >/dev/null 2>&1; then
         echo -e "${RED}✗ npm is required. Please install it first.${RESET}"
         exit 1
    fi

    echo -e "${BLUE}❯ Updating npm...${RESET}"
    npm install -g npm@latest || echo -e "${YELLOW}⚠ Failed to update npm, continuing...${RESET}"

    echo -e "${BLUE}❯ Cleaning npm cache...${RESET}"
    npm cache clean --force || echo -e "${YELLOW}⚠ Failed to clean npm cache, continuing...${RESET}"


    echo -e "${BLUE}❯ Installing Wrangler v4.12.0...${RESET}"
    WRANGLER_INSTALLED=0
    for attempt in {1..3}; do
        echo -e "${YELLOW}ℹ Attempt \$attempt to install Wrangler...${RESET}"
        if npm install -g wrangler@4.12.0; then
            echo -e "${GREEN}✓ Wrangler installed successfully.${RESET}"
            WRANGLER_INSTALLED=1
            break
        fi
        if [ \$attempt -eq 3 ]; then
             echo -e "${RED}✗ Failed to install Wrangler after 3 attempts.${RESET}"
             exit 1
        fi
        echo -e "${YELLOW}ℹ Retrying npm install in 5 seconds...${RESET}"
        sleep 5
    done

    if [ \$WRANGLER_INSTALLED -eq 1 ]; then
        WRANGLER_VERSION=\$(npx wrangler --version 2>/dev/null)
        if [[ "\$WRANGLER_VERSION" != *"4.12.0"* ]]; then
            echo -e "${RED}✗ Installed Wrangler version \$WRANGLER_VERSION does not match required version 4.12.0.${RESET}"
            exit 1
        else
            echo -e "${GREEN}✓ Wrangler version 4.12.0 verified.${RESET}"
        fi
    else
         echo -e "${RED}✗ Wrangler was not installed successfully.${RESET}"
         exit 1
    fi


    INSTALL_DIR="$HOME/.bpb-terminal-wizard"
    BINARY_NAME="BPB-Terminal-Wizard"
    RELEASE_URL="https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v1.1/BPB-Terminal-Wizard-${OS_TYPE}-${ARCH_TYPE}"

    echo -e "${BLUE}❯ Preparing BPB Terminal Wizard directory...${RESET}"
    mkdir -p "$INSTALL_DIR" || { echo -e "${RED}✗ Could not create directory $INSTALL_DIR${RESET}"; exit 1; }
    cd "$INSTALL_DIR" || { echo -e "${RED}✗ Could not change to directory $INSTALL_DIR${RESET}"; exit 1; }

    echo -e "${BLUE}❯ Downloading $BINARY_NAME for ${OS_TYPE}-${ARCH_TYPE}...${RESET}"
    curl -L --fail "$RELEASE_URL" -o "$BINARY_NAME" || { echo -e "${RED}✗ Error downloading $BINARY_NAME${RESET}"; exit 1; }
    chmod +x "$BINARY_NAME" || { echo -e "${RED}✗ Error making $BINARY_NAME executable${RESET}'; exit 1; }"

    echo -e "${BLUE}❯ Running BPB Terminal Wizard...${RESET}"
    ./"$BINARY_NAME" || { echo -e "${RED}✗ Error running $BINARY_NAME${RESET}"; exit 1; }
fi

echo -e "${GREEN}✓ Installation script finished.${RESET}"