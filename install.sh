#!/bin/bash

REQUIRED_NODE_VERSION_MAJOR=18
WRANGLER_VERSION="4.12.0"
PROJECT_NAME="BPB-Terminal-Wizard"
RELEASE_TAG="v1.2"

RESET='\033[0m'
BOLD='\033[1m'
BLUE='\033[0;34m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
BOLD_BLUE='\033[1;34m'
BOLD_GREEN='\033[1;32m'

print_header() {
    echo -e "${BOLD_BLUE}╔═══════════════════════════════════════════${RESET}"
    echo -e "${BOLD_BLUE}║ ${RESET}${BOLD}${PROJECT_NAME}${RESET}"
    echo -e "${BOLD_BLUE}║ ${RESET}"
    echo -e "${BOLD_BLUE}║ ${CYAN}A tool to deploy BPB Panel easily${RESET}"
    echo -e "${BOLD_BLUE}║ ${CYAN}Created by ${RESET}${BOLD_GREEN}Anonymous${RESET}${CYAN} with thanks to ${RESET}${BOLD_GREEN}BPB${RESET}"
    echo -e "${BOLD_BLUE}╚═══════════════════════════════════════════${RESET}"
    echo ""
}

print_info() {
    echo -e "${BLUE}❯ $1${RESET}"
}

print_success() {
    echo -e "${GREEN}✓ $1${RESET}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${RESET}"
}

print_error() {
    echo -e "${RED}✗ $1${RESET}"
}

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

check_node_version() {
    if ! command_exists node; then
        print_error "Node.js is not installed."
        return 1
    fi
    local version=$(node -v)
    local major_version=$(echo "$version" | sed 's/v//' | cut -d. -f1)
    if [[ "$major_version" -lt "$REQUIRED_NODE_VERSION_MAJOR" ]]; then
        print_error "Node.js version is too old ($version). Version $REQUIRED_NODE_VERSION_MAJOR or higher is required."
        return 1
    fi
    print_success "Node.js version ($version) is compatible."
    return 0
}

install_deps_apt() {
    print_info "Updating package lists..."
    if ! sudo apt update; then
        print_error "Failed to update package lists. Please check your internet connection and permissions."
        return 1
    fi

    print_info "Upgrading installed packages (this may take a while)..."
    if ! sudo apt upgrade -y; then
        print_warning "Could not upgrade all packages, but continuing installation..."
    fi

    print_info "Installing required packages (curl, wget, bash, npm, git)..."
    if ! sudo apt install -y curl wget bash npm git; then
        print_error "Failed to install essential packages. Please check apt logs."
        return 1
    fi

    if ! check_node_version; then
        print_warning "Attempting to install/update Node.js using NodeSource..."
        local node_pkg_status=$(dpkg -s nodejs 2>/dev/null | grep Status)
        if [[ -n "$node_pkg_status" && "$node_pkg_status" == *install* ]]; then
             print_info "Removing existing Node.js package before using NodeSource..."
             sudo apt remove nodejs -y
             sudo apt autoremove -y
        fi

        curl -fsSL "https://deb.nodesource.com/setup_${REQUIRED_NODE_VERSION_MAJOR}.x" | sudo -E bash -
        if ! sudo apt install -y nodejs; then
             print_error "Failed to install Node.js $REQUIRED_NODE_VERSION_MAJOR. Please install it manually."
             return 1
        fi
        if ! check_node_version; then
             print_error "Node.js installation failed or version is still incorrect after NodeSource setup."
             return 1
        fi
    fi

    print_info "Updating npm to the latest version..."
    if ! sudo npm install -g npm@latest; then
        print_warning "Failed to update npm. Continuing with the current version."
    fi

    return 0
}

install_wrangler() {
    local install_attempts=3
    local attempt=1

    print_info "Cleaning npm cache..."
    sudo npm cache clean --force || print_warning "Failed to clean npm cache. Continuing..."

    print_info "Installing Wrangler version $WRANGLER_VERSION..."
    while [ $attempt -le $install_attempts ]; do
        print_info "Attempt $attempt of $install_attempts to install Wrangler..."
        if sudo npm install -g "wrangler@${WRANGLER_VERSION}"; then
            print_success "Wrangler installed successfully."
            local installed_version=$(npx wrangler --version 2>/dev/null || wrangler --version 2>/dev/null)
             if [[ "$installed_version" == *"$WRANGLER_VERSION"* ]]; then
                print_success "Wrangler version $WRANGLER_VERSION verified."
                 print_info "Attempting to disable Wrangler telemetry..."
                 (npx wrangler telemetry disable || wrangler telemetry disable) > /dev/null 2>&1 || print_warning "Could not disable Wrangler telemetry. Continuing..."
                return 0
            else
                print_error "Wrangler installed, but version verification failed. Expected '$WRANGLER_VERSION', found '$installed_version'."
                return 1
            fi
        fi
        if [ $attempt -eq $install_attempts ]; then
            print_error "Failed to install Wrangler after $install_attempts attempts. Please check npm logs (~/.npm/_logs/) or try installing manually: sudo npm install -g wrangler@$WRANGLER_VERSION"
            return 1
        fi
        print_warning "Wrangler installation attempt $attempt failed. Retrying in 5 seconds..."
        sleep 5
        ((attempt++))
    done
}

download_and_run_wizard() {
    local os_type=$1
    local arch_type=$2
    local install_dir="$HOME/.bpb-terminal-wizard"
    local binary_name="${PROJECT_NAME}"
    local release_url="https://github.com/4n0nymou3/${PROJECT_NAME}/releases/download/${RELEASE_TAG}/${PROJECT_NAME}-${os_type}-${arch_type}"

    print_info "Preparing ${PROJECT_NAME} directory..."
    mkdir -p "$install_dir"
    if ! cd "$install_dir"; then
        print_error "Could not change to directory $install_dir"
        return 1
    fi

    print_info "Downloading $binary_name for ${os_type}-${arch_type}..."
    if ! curl -L --fail "$release_url" -o "$binary_name"; then
        print_error "Error downloading $binary_name from $release_url. Check the URL and your connection."
        return 1
    fi

    chmod +x "$binary_name"
    print_success "$binary_name downloaded successfully."

    print_info "Running ${PROJECT_NAME}..."
    if ./ "$binary_name"; then
        print_success "${PROJECT_NAME} finished successfully."
        return 0
    else
        print_error "${PROJECT_NAME} exited with an error."
        return 1
    fi
}

print_header

if ! command_exists sudo; then
    print_error "'sudo' command not found. Please install sudo or run parts of this script as root."
fi


if [ -d "/data/data/com.termux" ] && [ ! -f "/etc/os-release" ]; then
    print_info "Detected Termux environment. Setting up Ubuntu via proot-distro..."

    print_info "Updating Termux packages..."
    pkg update -y && pkg upgrade -y

    print_info "Installing Termux dependencies (proot-distro, wget, curl)..."
    pkg install termux-tools proot-distro wget curl -y

    if ! proot-distro list | grep -q ubuntu; then
        print_info "Installing Ubuntu distribution (this may take a while)..."
        if ! proot-distro install ubuntu; then
            print_error "Failed to install Ubuntu distribution via proot-distro."
            exit 1
        fi
    else
         print_success "Ubuntu distribution already installed."
    fi

    print_info "Running setup within Ubuntu environment..."
    proot-distro login ubuntu -- bash -c "
        RED='\033[0;31m'
        GREEN='\033[0;32m'
        BLUE='\033[0;34m'
        YELLOW='\033[0;33m'
        RESET='\033[0m'
        echo -e \"${BLUE}❯ Running inside Ubuntu proot environment...${RESET}\"

        apt update && apt upgrade -y
        apt install -y curl wget bash npm git sudo

        node_version_ok=false
        if command -v node >/dev/null 2>&1; then
            version=\$(node -v)
            major_version=\$(echo \"\$version\" | sed 's/v//' | cut -d. -f1)
            if [[ \"\$major_version\" -ge $REQUIRED_NODE_VERSION_MAJOR ]]; then
                echo -e \"${GREEN}✓ Node.js version (\$version) is compatible.${RESET}\"
                node_version_ok=true
            else
                 echo -e \"${YELLOW}⚠ Node.js version is too old (\$version). Upgrading...${RESET}\"
                 apt remove nodejs -y
                 apt autoremove -y
            fi
        else
            echo -e \"${YELLOW}⚠ Node.js not found. Installing...${RESET}\"
        fi

        if [ \"\$node_version_ok\" = false ]; then
             echo -e \"${BLUE}❯ Installing/Updating Node.js using NodeSource...${RESET}\"
             curl -fsSL https://deb.nodesource.com/setup_${REQUIRED_NODE_VERSION_MAJOR}.x | bash -
             if ! apt install -y nodejs; then
                  echo -e \"${RED}✗ Failed to install Node.js ${REQUIRED_NODE_VERSION_MAJOR}. Please check logs.${RESET}\"
                  exit 1
             fi
        fi

        echo -e \"${BLUE}❯ Installing Wrangler v$WRANGLER_VERSION globally...${RESET}\"
        npm cache clean --force || echo -e \"${YELLOW}⚠ Failed to clean npm cache.${RESET}\"
        install_attempts=3
        attempt=1
        while [ \$attempt -le \$install_attempts ]; do
            echo -e \"${YELLOW}ℹ Attempt \$attempt to install Wrangler...${RESET}\"
            if npm install -g wrangler@${WRANGLER_VERSION}; then
                 echo -e \"${GREEN}✓ Wrangler installed successfully.${RESET}\"
                 installed_version=\$(wrangler --version)
                 if [[ \"\$installed_version\" == *\"$WRANGLER_VERSION\"* ]]; then
                     echo -e \"${GREEN}✓ Wrangler version $WRANGLER_VERSION verified.${RESET}\"
                      echo -e \"${BLUE}❯ Attempting to disable Wrangler telemetry...${RESET}\"
                      wrangler telemetry disable > /dev/null 2>&1 || echo -e \"${YELLOW}⚠ Could not disable Wrangler telemetry.${RESET}\"
                     break
                 else
                      echo -e \"${RED}✗ Wrangler installed, but version mismatch. Expected $WRANGLER_VERSION, Got \$installed_version.${RESET}\"
                      exit 1
                 fi
            fi
             if [ \$attempt -eq \$install_attempts ]; then
                 echo -e \"${RED}✗ Failed to install Wrangler after \$install_attempts attempts.${RESET}\"
                 exit 1
             fi
             echo -e \"${YELLOW}ℹ Retrying npm install in 5 seconds...${RESET}\"
             sleep 5
             ((attempt++))
         done


        INSTALL_DIR=\"/root/.bpb-terminal-wizard\"
        BINARY_NAME=\"$PROJECT_NAME\"
        RELEASE_URL=\"https://github.com/4n0nymou3/${PROJECT_NAME}/releases/download/${RELEASE_TAG}/${PROJECT_NAME}-linux-arm64\"

        echo -e \"${BLUE}❯ Preparing ${PROJECT_NAME} directory in Ubuntu...${RESET}\"
        mkdir -p \"\$INSTALL_DIR\"
        cd \"\$INSTALL_DIR\" || { echo -e \"${RED}✗ Could not change to directory \$INSTALL_DIR${RESET}\"; exit 1; }

        echo -e \"${BLUE}❯ Downloading $BINARY_NAME for linux-arm64...${RESET}\"
        curl -L --fail \"\$RELEASE_URL\" -o \"\$BINARY_NAME\" || { echo -e \"${RED}✗ Error downloading $BINARY_NAME${RESET}\"; exit 1; }
        chmod +x \"\$BINARY_NAME\"
        echo -e \"${GREEN}✓ $BINARY_NAME downloaded successfully.${RESET}\"

        echo -e \"${BLUE}❯ Running ${PROJECT_NAME}...${RESET}\"
        ./\"\$BINARY_NAME\"
        exit \$?
    "
    termux_exit_code=$?
    if [ $termux_exit_code -eq 0 ]; then
        print_success "Termux setup completed successfully."
    else
        print_error "Termux setup failed with exit code $termux_exit_code."
    fi
    exit $termux_exit_code

else
    OS=$(uname -s)
    ARCH=$(uname -m)
    OS_TYPE=""
    ARCH_TYPE=""

    case "$OS" in
        Linux*)  OS_TYPE="linux" ;;
        Darwin*) OS_TYPE="darwin" ;;
        *)       print_error "Unsupported OS: $OS"; exit 1 ;;
    esac
    case "$ARCH" in
        x86_64)  ARCH_TYPE="amd64" ;;
        aarch64) ARCH_TYPE="arm64" ;;
        arm64)   ARCH_TYPE="arm64" ;;
        *)       print_error "Unsupported architecture: $ARCH"; exit 1 ;;
    esac

    print_info "Detected environment: ${OS_TYPE}-${ARCH_TYPE}"

    print_info "Checking dependencies..."

    if ! command_exists git; then
        print_error "git is required. Please install it first."
        if [ "$OS_TYPE" == "linux" ]; then
             print_info "On Debian/Ubuntu: sudo apt install git"
             print_info "On Fedora/CentOS: sudo dnf install git"
        elif [ "$OS_TYPE" == "darwin" ]; then
             print_info "On macOS: Install Xcode Command Line Tools (xcode-select --install) or use Homebrew (brew install git)."
        fi
        exit 1
    fi
    print_success "git is installed."

    if ! command_exists npm; then
        print_error "npm is required. Please install Node.js (which includes npm)."
         if [ "$OS_TYPE" == "linux" ]; then
             print_info "On Debian/Ubuntu: sudo apt install npm (may need NodeSource setup for latest versions, see script logic)"
             print_info "On Fedora/CentOS: sudo dnf install npm"
         elif [ "$OS_TYPE" == "darwin" ]; then
             print_info "On macOS: Install Node.js from nodejs.org or use Homebrew (brew install node)."
         fi
        exit 1
    fi
     print_success "npm is installed."

    if ! check_node_version; then
         if [ "$OS_TYPE" == "linux" ]; then
             print_info "Consider using NodeSource to update Node.js: curl -fsSL https://deb.nodesource.com/setup_${REQUIRED_NODE_VERSION_MAJOR}.x | sudo -E bash - && sudo apt install -y nodejs"
         elif [ "$OS_TYPE" == "darwin" ]; then
             print_info "On macOS: Use Homebrew (brew upgrade node) or download the latest installer from nodejs.org."
         fi
         exit 1
     fi

    if [ "$OS_TYPE" == "linux" ]; then
        if command_exists apt; then
            print_info "Detected Debian/Ubuntu based Linux. Ensuring dependencies are installed/updated..."
            if ! install_deps_apt; then
                print_error "Dependency installation failed on Linux (apt)."
                exit 1
            fi
             print_success "Linux (apt) dependencies checked/installed."
        else
            print_warning "Non-apt based Linux detected. Please ensure curl, wget, bash, git, Node.js v${REQUIRED_NODE_VERSION_MAJOR}+, and npm are installed manually."
        fi
    elif [ "$OS_TYPE" == "darwin" ]; then
         print_info "Detected macOS. Ensure dependencies (git, Node.js v${REQUIRED_NODE_VERSION_MAJOR}+, npm) are installed."
         print_info "You can use Homebrew (brew install git node) or download installers manually."
         if ! command_exists brew; then
             print_warning "Homebrew not detected. Cannot automatically install/update macOS dependencies."
         fi
    fi


    print_info "Checking/Installing Wrangler..."
    if ! install_wrangler; then
         exit 1
    fi


    if ! download_and_run_wizard "$OS_TYPE" "$ARCH_TYPE"; then
        exit 1
    fi

fi

print_success "Installation script finished."
exit 0
