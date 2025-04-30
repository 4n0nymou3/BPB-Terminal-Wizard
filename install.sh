#!/bin/bash

set -e

VERSION="1.4"

RESET='\033[0m'
BOLD='\033[1m'
BLUE='\033[0;34m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
BOLD_BLUE='\033[1;34m'
BOLD_GREEN='\033[1;32m'

INSTALL_DIR="$HOME/.bpb-terminal-wizard"
MAX_RETRIES=3
RETRY_DELAY=5

print_header() {
    echo -e "${BOLD_BLUE}╔═══════════════════════════════════════════${RESET}"
    echo -e "${BOLD_BLUE}║ ${RESET}${BOLD}BPB Terminal Wizard v${VERSION}${RESET}"
    echo -e "${BOLD_BLUE}║ ${RESET}"
    echo -e "${BOLD_BLUE}║ ${CYAN}A tool to deploy BPB Panel easily${RESET}"
    echo -e "${BOLD_BLUE}║ ${CYAN}Created by ${RESET}${BOLD_GREEN}Anonymous${RESET}${CYAN} with thanks to ${RESET}${BOLD_GREEN}BPB${RESET}"
    echo -e "${BOLD_BLUE}╚═══════════════════════════════════════════${RESET}"
    echo ""
}

# These functions are defined globally for macOS and Linux(Ubuntu) execution paths
run_with_retry() {
    local cmd="$1"
    local attempt=1
    while [ $attempt -le $MAX_RETRIES ]; do
        echo -e "${YELLOW}ℹ Attempt $attempt: Running '$cmd'...${RESET}"
        if eval "$cmd"; then
            echo -e "${GREEN}✓ Command successful.${RESET}"
            return 0
        fi
        if [ $attempt -eq $MAX_RETRIES ]; then
            echo -e "${RED}✗ Command failed after $MAX_RETRIES attempts: '$cmd'${RESET}"
            return 1
        fi
        echo -e "${YELLOW}ℹ Retrying in $RETRY_DELAY seconds...${RESET}"
        sleep $RETRY_DELAY
        attempt=$((attempt + 1))
    done
}

check_command() {
    local cmd_name="$1"
    if ! command -v "$cmd_name" >/dev/null 2>&1; then
        echo -e "${YELLOW}ℹ $cmd_name not found.${RESET}"
        return 1
    fi
    echo -e "${GREEN}✓ $cmd_name found.${RESET}"
    return 0
}

check_node_version() {
    if ! check_command "node"; then
        return 1
    fi
    local node_version major_version
    node_version=$(node -v | cut -d'v' -f2)
    major_version=$(echo "$node_version" | cut -d'.' -f1)
    if [ "$major_version" -lt 18 ]; then
        echo -e "${YELLOW}ℹ Node.js version v$node_version is too old (requires v18+).${RESET}"
        return 1
    fi
    echo -e "${GREEN}✓ Node.js version v$node_version is compatible.${RESET}"
    return 0
}

check_npm_version() {
    if ! check_command "npm"; then
        return 1
    fi
    echo -e "${GREEN}✓ npm detected.${RESET}"
    return 0
}

install_latest_npm() {
    echo -e "${BLUE}❯ Installing/Updating npm to latest version...${RESET}"
    if run_with_retry "npm install -g npm@latest"; then
        echo -e "${GREEN}✓ npm updated successfully.${RESET}"
    else
        echo -e "${RED}✗ Failed to update npm.${RESET}"
        return 1
    fi
}

check_wrangler_version() {
    if ! check_command "wrangler"; then
        return 1
    fi
    echo -e "${GREEN}✓ Wrangler detected.${RESET}"
    return 0
}

install_latest_wrangler() {
    echo -e "${BLUE}❯ Installing/Updating Wrangler...${RESET}"
    if run_with_retry "npm install -g wrangler"; then
        echo -e "${GREEN}✓ Wrangler installed/updated successfully.${RESET}"
    else
        echo -e "${RED}✗ Failed to install/update Wrangler.${RESET}"
        return 1
    fi
}

install_dependencies_ubuntu() {
    echo -e "${BLUE}❯ Updating package lists...${RESET}"
    apt-get update -y
    echo -e "${BLUE}❯ Upgrading existing packages...${RESET}"
    apt-get upgrade -y --with-new-pkgs # Added flag for robustness
    echo -e "${BLUE}❯ Installing base dependencies (curl, wget, bash, git)...${RESET}"
    apt-get install -y curl wget bash git

    if ! check_node_version; then
        echo -e "${BLUE}❯ Installing Node.js (latest LTS)...${RESET}"
        curl -fsSL https://deb.nodesource.com/setup_current.x | bash -
        apt-get install -y nodejs
    fi

    if ! check_npm_version || ! install_latest_npm; then
        exit 1
    fi

    echo -e "${BLUE}❯ Cleaning npm cache...${RESET}"
    npm cache clean --force || echo -e "${YELLOW}ℹ Warning: Failed to clean npm cache.${RESET}"

    if ! check_wrangler_version || ! install_latest_wrangler; then
        exit 1
    fi
}

install_dependencies_macos() {
    if ! check_command "brew"; then
        echo -e "${RED}✗ Homebrew is not installed. Please install Homebrew first (https://brew.sh/).${RESET}"
        exit 1
    fi
    echo -e "${BLUE}❯ Updating Homebrew...${RESET}"
    brew update

    if ! check_node_version; then
        echo -e "${BLUE}❯ Installing Node.js...${RESET}"
        brew install node
    fi
    if ! check_command "git"; then
        echo -e "${BLUE}❯ Installing Git...${RESET}"
        brew install git
    fi

    if ! check_npm_version || ! install_latest_npm; then
        exit 1
    fi

    echo -e "${BLUE}❯ Cleaning npm cache...${RESET}"
    npm cache clean --force || echo -e "${YELLOW}ℹ Warning: Failed to clean npm cache.${RESET}"

    if ! check_wrangler_version || ! install_latest_wrangler; then
        exit 1
    fi
}

download_wizard() {
    local os_type="$1"
    local arch_type="$2"
    local binary_name="BPB-Terminal-Wizard"
    local release_url="https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v${VERSION}/BPB-Terminal-Wizard-${os_type}-${arch_type}"

    echo -e "${BLUE}❯ Preparing BPB Terminal Wizard directory: $INSTALL_DIR${RESET}"
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR" || { echo -e "${RED}✗ Could not change to directory $INSTALL_DIR${RESET}"; exit 1; }

    echo -e "${BLUE}❯ Downloading $binary_name for ${os_type}-${arch_type}...${RESET}"
    if run_with_retry "curl -fL --retry $MAX_RETRIES --retry-delay $RETRY_DELAY '$release_url' -o '$binary_name'"; then
        chmod +x "$binary_name"
        echo -e "${GREEN}✓ Download and setup successful.${RESET}"
    else
        echo -e "${RED}✗ Failed to download $binary_name.${RESET}"
        exit 1
    fi
}

run_wizard() {
    local binary_path="$1"
    echo -e "${BLUE}❯ Running BPB Terminal Wizard...${RESET}"
    "$binary_path"
}


# --- Main Script Logic ---

print_header

if [ -d "/data/data/com.termux" ] && [ ! -f "/etc/os-release" ]; then
    echo -e "${YELLOW}ℹ Detected Termux environment. Setting up Ubuntu via proot-distro...${RESET}"
    pkg update -y && pkg upgrade -y
    pkg install termux-tools proot-distro wget curl -y

    if ! proot-distro list | grep -q ubuntu; then
        echo -e "${BLUE}❯ Installing Ubuntu distribution...${RESET}"
        proot-distro install ubuntu
    else
        echo -e "${GREEN}✓ Ubuntu distribution already installed.${RESET}"
    fi

    echo -e "${BLUE}❯ Logging into Ubuntu to install dependencies and run Wizard...${RESET}"

    # Define the script to be executed inside proot-distro
    # Pass necessary variables and define functions directly within the script block
    PROOT_SCRIPT=$(cat <<EOF
set -e

# Define variables passed from the outer script
VERSION='${VERSION}'
RESET='${RESET}'
BOLD='${BOLD}'
BLUE='${BLUE}'
GREEN='${GREEN}'
CYAN='${CYAN}'
YELLOW='${YELLOW}'
RED='${RED}'
BOLD_BLUE='${BOLD_BLUE}'
BOLD_GREEN='${BOLD_GREEN}'
MAX_RETRIES=${MAX_RETRIES}
RETRY_DELAY=${RETRY_DELAY}
export INSTALL_DIR='/root/.bpb-terminal-wizard' # Define install dir inside proot

# --- Redefine Helper Functions directly inside ---
# These definitions are scoped to this 'bash -c' execution

run_with_retry() {
    local cmd="\$1"
    local attempt=1
    while [ \$attempt -le \$MAX_RETRIES ]; do
        echo -e "\${YELLOW}ℹ Attempt \$attempt: Running '\$cmd'...\${RESET}"
        if eval "\$cmd"; then
            echo -e "\${GREEN}✓ Command successful.\${RESET}"
            return 0
        fi
        if [ \$attempt -eq \$MAX_RETRIES ]; then
            echo -e "\${RED}✗ Command failed after \$MAX_RETRIES attempts: '\$cmd'\${RESET}"
            return 1
        fi
        echo -e "\${YELLOW}ℹ Retrying in \$RETRY_DELAY seconds...\${RESET}"
        sleep \$RETRY_DELAY
        attempt=\$((attempt + 1))
    done
}

check_command() {
    local cmd_name="\$1"
    if ! command -v "\$cmd_name" >/dev/null 2>&1; then
        echo -e "\${YELLOW}ℹ \$cmd_name not found.\${RESET}"
        return 1
    fi
    echo -e "\${GREEN}✓ \$cmd_name found.\${RESET}"
    return 0
}

check_node_version() {
    if ! check_command "node"; then
        return 1
    fi
    local node_version major_version
    node_version=\$(node -v | cut -d'v' -f2)
    major_version=\$(echo "\$node_version" | cut -d'.' -f1)
    if [ "\$major_version" -lt 18 ]; then
        echo -e "\${YELLOW}ℹ Node.js version v\$node_version is too old (requires v18+).\${RESET}"
        return 1
    fi
    echo -e "\${GREEN}✓ Node.js version v\$node_version is compatible.\${RESET}"
    return 0
}

check_npm_version() {
    if ! check_command "npm"; then
        return 1
    fi
    echo -e "\${GREEN}✓ npm detected.\${RESET}"
    return 0
}

install_latest_npm() {
    echo -e "\${BLUE}❯ Installing/Updating npm to latest version...\${RESET}"
    if run_with_retry "npm install -g npm@latest"; then
        echo -e "\${GREEN}✓ npm updated successfully.\${RESET}"
    else
        echo -e "\${RED}✗ Failed to update npm.\${RESET}"
        return 1
    fi
}

check_wrangler_version() {
    if ! check_command "wrangler"; then
        return 1
    fi
    # Check if wrangler is runnable via npx as well, sometimes global link fails
    if npx wrangler --version > /dev/null 2>&1; then
        echo -e "\${GREEN}✓ Wrangler detected (via npx).\${RESET}"
        return 0
    else
        echo -e "\${YELLOW}ℹ Wrangler command failed, trying npx wrangler install...\${RESET}"
        return 1 # Indicate check failed, needs install
    fi
}

install_latest_wrangler() {
    echo -e "\${BLUE}❯ Installing/Updating Wrangler... (using npm install -g)\${RESET}"
    # Try global install first, it's cleaner if it works
    if run_with_retry "npm install -g wrangler"; then
        echo -e "\${GREEN}✓ Wrangler installed/updated globally successfully.\${RESET}"
        # Verify wrangler command is now available
         if ! command -v wrangler > /dev/null 2>&1; then
             echo -e "\${YELLOW}⚠ Wrangler installed globally but command not found in PATH. Will rely on npx.\${RESET}"
         fi
         return 0
    else
        echo -e "\${RED}✗ Failed to install/update Wrangler globally. Ensure npm and Node.js are correctly set up.\${RESET}"
        return 1
    fi
}


install_dependencies_ubuntu() {
    echo -e "\${BLUE}❯ Updating package lists (inside proot)...\${RESET}"
    apt-get update -y
    echo -e "\${BLUE}❯ Upgrading existing packages (inside proot)...\${RESET}"
    apt-get upgrade -y --with-new-pkgs
    echo -e "\${BLUE}❯ Installing base dependencies (curl, wget, bash, git) (inside proot)...\${RESET}"
    apt-get install -y curl wget bash git

    if ! check_node_version; then
        echo -e "\${BLUE}❯ Installing Node.js (latest LTS) (inside proot)...\${RESET}"
        curl -fsSL https://deb.nodesource.com/setup_current.x | bash -
        apt-get install -y nodejs
    fi

    if ! check_npm_version || ! install_latest_npm; then
        exit 1
    fi

    echo -e "\${BLUE}❯ Cleaning npm cache (inside proot)...\${RESET}"
    npm cache clean --force || echo -e "\${YELLOW}ℹ Warning: Failed to clean npm cache (inside proot).\${RESET}"

    if ! check_wrangler_version; then
        if ! install_latest_wrangler; then
             exit 1
        fi
    fi
    # Final check using npx which should work even if global path isn't set
    if ! npx wrangler --version > /dev/null 2>&1; then
         echo -e "\${RED}✗ Wrangler still not runnable via npx after installation attempt.\${RESET}"
         exit 1
    fi
    echo -e "\${GREEN}✓ Wrangler confirmed runnable via npx.\${RESET}"

}

download_wizard() {
    local os_type="\$1"
    local arch_type="\$2"
    local binary_name="BPB-Terminal-Wizard"
    local release_url="https://github.com/4n0nymou3/BPB-Terminal-Wizard/releases/download/v\${VERSION}/BPB-Terminal-Wizard-\${os_type}-\${arch_type}"

    echo -e "\${BLUE}❯ Preparing BPB Terminal Wizard directory: \$INSTALL_DIR (inside proot)...\${RESET}"
    mkdir -p "\$INSTALL_DIR"
    cd "\$INSTALL_DIR" || { echo -e "\${RED}✗ Could not change to directory \$INSTALL_DIR (inside proot)\${RESET}"; exit 1; }

    echo -e "\${BLUE}❯ Downloading \$binary_name for \${os_type}-\${arch_type} (inside proot)...\${RESET}"
    # Use run_with_retry defined within this script block
    if run_with_retry "curl -fL --retry \$MAX_RETRIES --retry-delay \$RETRY_DELAY '\$release_url' -o '\$binary_name'"; then
        chmod +x "\$binary_name"
        echo -e "\${GREEN}✓ Download and setup successful (inside proot).\${RESET}"
    else
        echo -e "\${RED}✗ Failed to download \$binary_name (inside proot).\${RESET}"
        exit 1
    fi
}

run_wizard() {
    local binary_path="\$1"
    echo -e "\${BLUE}❯ Running BPB Terminal Wizard (inside proot)...\${RESET}"
    "\$binary_path"
}

# --- Main logic inside proot ---
install_dependencies_ubuntu
download_wizard linux arm64
run_wizard "\$INSTALL_DIR/BPB-Terminal-Wizard"

EOF
)
    # Execute the script within proot-distro
    proot-distro login ubuntu -- bash -c "$PROOT_SCRIPT"


elif [ "$(uname -s)" == "Darwin" ]; then
    echo -e "${YELLOW}ℹ Detected macOS environment.${RESET}"
    install_dependencies_macos

    ARCH_TYPE=$(uname -m)
    case "$ARCH_TYPE" in
        x86_64) ARCH_TYPE="amd64" ;;
        arm64)  ARCH_TYPE="arm64" ;;
        *)      echo -e "${RED}✗ Unsupported macOS architecture: $ARCH_TYPE${RESET}"; exit 1 ;;
    esac

    download_wizard darwin "$ARCH_TYPE"
    run_wizard "$INSTALL_DIR/BPB-Terminal-Wizard"

elif [ "$(uname -s)" == "Linux" ] && [ -f "/etc/os-release" ] && grep -q -i "ubuntu" /etc/os-release; then
    echo -e "${YELLOW}ℹ Detected Ubuntu Linux environment.${RESET}"
    install_dependencies_ubuntu

    ARCH_TYPE=$(uname -m)
    case "$ARCH_TYPE" in
        x86_64)      ARCH_TYPE="amd64" ;;
        aarch64|arm64) ARCH_TYPE="arm64" ;;
        *)           echo -e "${RED}✗ Unsupported Linux architecture: $ARCH_TYPE${RESET}"; exit 1 ;;
    esac

    download_wizard linux "$ARCH_TYPE"
    run_wizard "$INSTALL_DIR/BPB-Terminal-Wizard"

else
    OS=$(uname -s)
    ARCH=$(uname -m)
    echo -e "${RED}✗ Unsupported OS ($OS) or Linux distribution.${RESET}"
    echo -e "${YELLOW}ℹ This script currently supports macOS, Termux (via Ubuntu proot), and Ubuntu Linux.${RESET}"
    echo -e "${YELLOW}ℹ For other systems, please install Node.js (v18+), npm, and Wrangler manually, then download the appropriate binary from the releases page and run it.${RESET}"
    exit 1
fi

echo -e "${GREEN}✓ BPB Terminal Wizard installation script finished.${RESET}"