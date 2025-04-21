# BPB Terminal Wizard

A command-line tool to simplify the deployment of the BPB Panel on Cloudflare using Linux, macOS, or Termux. This tool automates the installation of dependencies, Cloudflare login, and panel deployment, making the process seamless, especially in the Termux environment.

## Installation

### Prerequisites
- A terminal environment (Linux, macOS, or Termux on Android).
- Internet connection for downloading dependencies.

### Steps for Termux
1. Update and upgrade Termux packages:
   pkg update && pkg upgrade
2. Run the installation script:
```bash
   curl -sSL https://raw.githubusercontent.com/4n0nymou3/BPB-Terminal-Wizard/main/install.sh | bash
```

### Steps for Linux/macOS
1. Ensure curl, wget, bash, and nodejs (version 18 or higher) are installed.
2. Run the installation script:
```bash
   curl -sSL https://raw.githubusercontent.com/4n0nymou3/BPB-Terminal-Wizard/main/install.sh | bash
```

## Usage
- The script will guide you through the installation process, including Cloudflare login and panel deployment.
- For Termux, the browser will automatically open for Cloudflare authentication. Press "Allow" and return to Termux to continue.
- Upon completion, the script provides a URL to access the deployed BPB Panel.

## License
This project is licensed under the GPL-3.0 License.

## Author
Created by [Anonymous](https://x.com/4n0nymou3) with thanks to BPB.
