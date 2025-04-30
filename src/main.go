package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type KVNamespace struct {
	Title string `json:"title"`
	ID    string `json:"id"`
}

const (
	workerURL        = "https://github.com/bia-pain-bache/BPB-Worker-Panel/releases/latest/download/worker.js"
	defaultProxyIP   = "bpb.yousef.isegaro.com"
	defaultFallback  = "speed.cloudflare.com"
	nodeMinMajorVer  = 18
	retryAttempts    = 3
	retryDelay       = 5 * time.Second
	loginTimeout     = 60 * time.Second
	randomDomainLen  = 32
	trojanPasswordLen = 12
	subPathLen       = 16
	kvNamePrefix     = "panel_kv_"
	kvNameRandLen    = 8
	kvNameCharset    = "abcdefghijklmnopqrstuvwxyz0123456789"
	domainCharset    = "abcdefghijklmnopqrstuvwxyz0123456789-"
	passwordCharset  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+[]{}|;:',.<>?"
	subPathCharset   = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@$&*_-+;:,."
)

var (
	red          = "\033[0;31m"
	green        = "\033[0;32m"
	yellow       = "\033[0;33m"
	blue         = "\033[0;34m"
	cyan         = "\033[0;36m"
	reset        = "\033[0m"
	bold         = "\033[1m"
	titlePrefix  = bold + cyan + "◆" + reset
	infoPrefix   = bold + blue + "❯" + reset
	warnPrefix   = bold + yellow + "⚠" + reset
	errorPrefix  = bold + red + "✗" + reset
	successPrefix= bold + green + "✓" + reset
)

type Config struct {
	DeployType         string
	ProjectName        string
	CustomDomain       string
	UUID               string
	TR_PASS            string
	PROXY_IP           string
	FALLBACK           string
	SUB_PATH           string
	KvID               string
	InstallDir         string
	SrcPath            string
	WorkerPath         string
	WranglerConfigPath string
	PanelURL           string
}

func main() {
	cfg := &Config{}
	flag.StringVar(&cfg.DeployType, "deploy", "1", "Deployment type: 1 for Workers, 2 for Pages")
	flag.Parse()

	if err := validateDeployType(cfg.DeployType); err != nil {
		failMessage(err.Error(), nil)
		return
	}

	if err := setupDirectories(cfg); err != nil {
		failMessage("Failed to setup directories", err)
		return
	}

	printTitle("Installing BPB Terminal Wizard...")

	if err := checkDependencies(cfg.InstallDir); err != nil {
		failMessage("Dependency check failed", err)
		return
	}

	successMessage("BPB Terminal Wizard dependencies are ready!")

	if err := cloudflareLogin(cfg.InstallDir); err != nil {
		failMessage("Cloudflare login failed", err)
		return
	}

	if err := configureWorkerSettings(cfg); err != nil {
		failMessage("Failed to configure worker settings", err)
		return
	}

	if err := downloadWorkerScript(cfg); err != nil {
		failMessage("Failed to download worker script", err)
		return
	}

	if err := createKVNamespace(cfg); err != nil {
		failMessage("Failed to create KV namespace", err)
		return
	}

	if err := buildWranglerConfig(cfg); err != nil {
		failMessage("Failed to build Wrangler configuration", err)
		return
	}

	if err := deployPanel(cfg); err != nil {
		failMessage("Failed to deploy panel", err)
		return
	}

	printSuccessPanelInfo(cfg.PanelURL)
}

func validateDeployType(deployType string) error {
	if deployType != "1" && deployType != "2" {
		return errors.New("invalid deploy type. Use -deploy=1 for Workers or -deploy=2 for Pages")
	}
	return nil
}

func setupDirectories(cfg *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	cfg.InstallDir = filepath.Join(homeDir, ".bpb-terminal-wizard")
	cfg.WranglerConfigPath = filepath.Join(cfg.InstallDir, "wrangler.toml")
	cfg.SrcPath = filepath.Join(cfg.InstallDir, "src")

	_ = os.Remove(cfg.WranglerConfigPath)
	if err := os.RemoveAll(cfg.SrcPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("deleting old src directory: %w", err)
	}

	if err := os.MkdirAll(cfg.InstallDir, 0750); err != nil {
		return fmt.Errorf("creating install directory %s: %w", cfg.InstallDir, err)
	}
	if err := os.Mkdir(cfg.SrcPath, 0750); err != nil {
		return fmt.Errorf("creating src directory %s: %w", cfg.SrcPath, err)
	}

	if cfg.DeployType == "1" {
		cfg.WorkerPath = filepath.Join(cfg.SrcPath, "worker.js")
	} else {
		cfg.WorkerPath = filepath.Join(cfg.SrcPath, "_worker.js")
	}

	return nil
}

func checkDependencies(installDir string) error {
	if err := checkNode(); err != nil {
		return fmt.Errorf("Node.js check failed: %w", err)
	}
	if err := checkNpm(); err != nil {
		return fmt.Errorf("npm check failed: %w", err)
	}

	printInfo("Installing Wrangler...")
	if err := checkWrangler(); err != nil {
		return fmt.Errorf("Wrangler check failed: %w", err)
	}
	if _, err := runCommand(installDir, "npm cache clean --force", 1); err != nil {
		printWarning("Could not clean npm cache, continuing anyway...")
	}
	output, err := runCommand(installDir, "npx wrangler --version", 1)
	if err != nil {
		return fmt.Errorf("failed to verify Wrangler installation: output: %s, error: %w", output, err)
	}
	return nil
}

func checkNode() error {
	output, err := exec.Command("node", "-v").Output()
	if err != nil {
		return fmt.Errorf("node command failed: %w", err)
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(output)), "v")
	versionParts := strings.Split(version, ".")
	if len(versionParts) < 1 {
		return fmt.Errorf("invalid Node.js version format: %s", version)
	}
	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return fmt.Errorf("cannot parse Node.js major version '%s': %w", versionParts[0], err)
	}
	if major < nodeMinMajorVer {
		return fmt.Errorf("Node.js version %s is too old, requires v%d or higher", version, nodeMinMajorVer)
	}
	return nil
}

func checkNpm() error {
	_, err := exec.Command("npm", "-v").Output()
	if err != nil {
		return fmt.Errorf("npm command failed: %w", err)
	}
	return nil
}

func checkWrangler() error {
	_, err := exec.Command("npx", "wrangler", "--version").Output()
	if err != nil {
		return fmt.Errorf("wrangler command failed: %w", err)
	}
	return nil
}

func cloudflareLogin(installDir string) error {
	printTitle("Starting Cloudflare login process...")
	for {
		cmd := exec.Command("sh", "-c", "npx wrangler login")
		cmd.Dir = installDir
		var stdoutBuf bytes.Buffer
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Start(); err != nil {
			printError("Error starting Cloudflare login", err)
			continue
		}

		oauthURL, err := waitForOAuthURL(&stdoutBuf)
		if err != nil {
			_ = cmd.Process.Kill()
			return fmt.Errorf("waiting for OAuth URL: %w", err)
		}

		printInfo(fmt.Sprintf("Found OAuth URL: %s%s%s", blue, oauthURL, reset))
		if err := openURL(oauthURL); err != nil {
			printWarning(fmt.Sprintf("Could not open browser automatically.\nPlease open this URL manually: %s%s%s", blue, oauthURL, reset))
		} else {
			printInfo(fmt.Sprintf("Browser opened with URL: %s%s%s", blue, oauthURL, reset))
		}

		if err := cmd.Wait(); err != nil {
			printError("Error during Cloudflare login process", err)
			continue
		}

		if _, err := runCommand(installDir, "npx wrangler telemetry disable", 1); err != nil {
			printWarning("Could not disable telemetry, continuing anyway...")
		}

		successMessage("Successfully logged into Cloudflare!")
		break
	}
	return nil
}

func waitForOAuthURL(stdoutBuf *bytes.Buffer) (string, error) {
	timeout := time.After(loginTimeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return "", errors.New("timeout waiting for OAuth URL")
		case <-ticker.C:
			url, err := extractOAuthURL(stdoutBuf.String())
			if err == nil {
				return url, nil
			}
		}
	}
}

func configureWorkerSettings(cfg *Config) error {
	printTitle("Configuring Worker settings...")
	printInfo(fmt.Sprintf("Using deployment type: %s%s%s", bold+green, map[string]string{"1": "Workers", "2": "Pages"}[cfg.DeployType], reset))
	if cfg.DeployType == "2" {
		printWarning(fmt.Sprintf("With %sPages%s, you cannot modify settings later from Cloudflare dashboard.", bold+green, reset))
		printWarning(fmt.Sprintf("With %sPages%s, it may take up to 5 minutes to access the panel.", bold+green, reset))
	}

	if err := setProjectName(cfg); err != nil {
		return err
	}
	if err := setUUID(cfg); err != nil {
		return err
	}
	if err := setTrojanPassword(cfg); err != nil {
		return err
	}
	if err := setProxyIP(cfg); err != nil {
		return err
	}
	if err := setFallbackDomain(cfg); err != nil {
		return err
	}
	if err := setSubscriptionPath(cfg); err != nil {
		return err
	}

	return nil
}

func setProjectName(cfg *Config) error {
	for {
		cfg.ProjectName = generateRandomDomain(randomDomainLen)
		printInfo(fmt.Sprintf("\nGenerated worker name (%sSubdomain%s): %s%s%s", bold+green, reset, cyan, cfg.ProjectName, reset))
		successMessage("Using generated worker name.")

		printInfo("\nChecking domain availability...")
		isAvailable, err := isWorkerAvailable(cfg.InstallDir, cfg.ProjectName, cfg.DeployType)
		if err != nil {
			printWarning(fmt.Sprintf("Could not check domain availability, assuming it's available: %v", err))
			break
		}
		if !isAvailable {
			printWarning("Domain is not available, generating a new one...")
			continue
		}
		successMessage("Domain is available!")
		break
	}
	return nil
}

func setUUID(cfg *Config) error {
	cfg.UUID = uuid.NewString()
	printInfo(fmt.Sprintf("\nGenerated %sUUID%s: %s%s%s", bold+green, reset, cyan, cfg.UUID, reset))
	successMessage("Using generated UUID.")
	return nil
}

func setTrojanPassword(cfg *Config) error {
	cfg.TR_PASS = generateRandomString(passwordCharset, trojanPasswordLen, false)
	printInfo(fmt.Sprintf("\nGenerated %sTrojan password%s: %s%s%s", bold+green, reset, cyan, cfg.TR_PASS, reset))
	successMessage("Using generated Trojan password.")
	return nil
}

func setProxyIP(cfg *Config) error {
	cfg.PROXY_IP = defaultProxyIP
	printInfo(fmt.Sprintf("\nDefault %sProxy IP%s: %s%s%s", bold+green, reset, cyan, cfg.PROXY_IP, reset))
	successMessage("Using default Proxy IP.")
	return nil
}

func setFallbackDomain(cfg *Config) error {
	cfg.FALLBACK = defaultFallback
	printInfo(fmt.Sprintf("\nDefault %sFallback domain%s: %s%s%s", bold+green, reset, cyan, cfg.FALLBACK, reset))
	successMessage("Using default Fallback domain.")
	return nil
}

func setSubscriptionPath(cfg *Config) error {
	cfg.SUB_PATH = generateRandomString(subPathCharset, subPathLen, false)
	printInfo(fmt.Sprintf("\nGenerated %sSubscription path%s: %s%s%s", bold+green, reset, cyan, cfg.SUB_PATH, reset))
	successMessage("Using generated Subscription path.")
	return nil
}

func downloadWorkerScript(cfg *Config) error {
	printTitle(fmt.Sprintf("Downloading %sworker.js%s...", bold+green, reset))
	err := downloadFileWithRetry(workerURL, cfg.WorkerPath, retryAttempts)
	if err != nil {
		return fmt.Errorf("downloading worker script from %s: %w", workerURL, err)
	}
	successMessage("Worker downloaded successfully!")
	return nil
}

func createKVNamespace(cfg *Config) error {
	printWarning("\nThis program creates a new KV namespace each time it runs.\n   Check your Cloudflare account and delete unused KV namespaces to avoid limits.")
	printTitle("Creating KV namespace...")

	var kvErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		kvName := fmt.Sprintf("%s%s", kvNamePrefix, generateRandomString(kvNameCharset, kvNameRandLen, false))
		command := fmt.Sprintf("npx wrangler kv namespace create %s --json", kvName)
		output, err := runCommand(cfg.InstallDir, command, 1)

		if err != nil {
			kvErr = fmt.Errorf("creating KV on attempt %d: %w. Output: %s", attempt, err, output)
			if strings.Contains(output, "fetch failed") && attempt < retryAttempts {
				printInfo(fmt.Sprintf("Retrying after %v...", retryDelay))
				time.Sleep(retryDelay)
				continue
			}
			failMessage(fmt.Sprintf("Error creating KV! Output: %s. Check logs at ~/.wrangler/logs/", output), err)
			continue
		}

		id, err := extractKvIDFromJSON(output)
		if err != nil {
			kvErr = fmt.Errorf("extracting KV ID on attempt %d: %w. Output: %s", attempt, err, output)
			failMessage(fmt.Sprintf("Error getting KV ID! Output: %s. Check logs at ~/.wrangler/logs/", output), err)
			continue
		}

		cfg.KvID = id
		successMessage("KV namespace created successfully!")
		return nil
	}

	return fmt.Errorf("failed to create KV namespace after %d attempts: %w", retryAttempts, kvErr)
}

func buildWranglerConfig(cfg *Config) error {
	printTitle("Building panel configuration (wrangler.toml)...")

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "name = \"%s\"\n", cfg.ProjectName)
	fmt.Fprintf(&buf, "compatibility_date = \"%s\"\n", time.Now().AddDate(0, 0, -1).Format("2006-01-02"))
	fmt.Fprintf(&buf, "compatibility_flags = [\"nodejs_compat\"]\n\n")

	if cfg.DeployType == "1" {
		fmt.Fprintf(&buf, "main = \"src/worker.js\"\n")
		fmt.Fprintf(&buf, "workers_dev = true\n\n")
	} else {
		fmt.Fprintf(&buf, "[site]\n")
		fmt.Fprintf(&buf, "bucket = \"./src\"\n\n")
		fmt.Fprintf(&buf, "pages_build_output_dir = \"./src\"\n\n")
	}

	fmt.Fprintf(&buf, "kv_namespaces = [\n")
	fmt.Fprintf(&buf, "  { binding = \"kv\", id = \"%s\" }\n", cfg.KvID)
	fmt.Fprintf(&buf, "]\n\n")

	fmt.Fprintf(&buf, "[vars]\n")
	fmt.Fprintf(&buf, "UUID = \"%s\"\n", cfg.UUID)
	fmt.Fprintf(&buf, "TR_PASS = \"%s\"\n", cfg.TR_PASS)
	fmt.Fprintf(&buf, "PROXY_IP = \"%s\"\n", cfg.PROXY_IP)
	fmt.Fprintf(&buf, "FALLBACK = \"%s\"\n", cfg.FALLBACK)
	fmt.Fprintf(&buf, "SUB_PATH = \"%s\"\n", cfg.SUB_PATH)

	if cfg.CustomDomain != "" {
		fmt.Fprintf(&buf, "\nroutes = [\n")
		fmt.Fprintf(&buf, "  { pattern = \"%s\", custom_domain = true }\n", cfg.CustomDomain)
		fmt.Fprintf(&buf, "]\n")
	}

	if err := os.WriteFile(cfg.WranglerConfigPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing config to %s: %w", cfg.WranglerConfigPath, err)
	}

	successMessage("Panel configuration built successfully!")
	return nil
}

func deployPanel(cfg *Config) error {
	var deployErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		printTitle(fmt.Sprintf("Deploying %sBPB Panel%s (Attempt %d)...", bold+blue, reset, attempt))

		var err error
		var output string
		var url string

		if cfg.DeployType == "1" {
			output, err = runCommand(cfg.InstallDir, fmt.Sprintf("npx wrangler deploy --config %s", cfg.WranglerConfigPath), 1)
			if err == nil {
				url, err = extractURL(output)
				if err == nil {
					cfg.PanelURL = url + "/panel"
					successMessage("Panel deployed successfully!")
					return nil
				}
				err = fmt.Errorf("extracting URL from output: %w", err)
			} else {
				err = fmt.Errorf("wrangler deploy command failed: %w. Output: %s", err, output)
			}
		} else {
			_, err = runCommand(cfg.InstallDir, fmt.Sprintf("npx wrangler pages project create %s --production-branch=main", cfg.ProjectName), 1)
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				err = fmt.Errorf("creating Pages project '%s': %w", cfg.ProjectName, err)
			} else {
				err = nil
			}

			if err == nil {
				_, err = runCommand(cfg.InstallDir, fmt.Sprintf("npx wrangler pages deploy ./src --project-name=%s --commit-dirty=true --branch=main", cfg.ProjectName), 1)
				if err == nil {
					cfg.PanelURL = "https://" + cfg.ProjectName + ".pages.dev/panel"
					successMessage("Panel deployed successfully!")
					return nil
				}
				err = fmt.Errorf("deploying Pages project '%s': %w", cfg.ProjectName, err)
			}
		}

		deployErr = err
		failMessage(fmt.Sprintf("Error deploying Panel! %v", err), nil)
		if attempt < retryAttempts {
			printInfo(fmt.Sprintf("Retrying deployment in %v...", retryDelay))
			time.Sleep(retryDelay)
		}
	}
	return fmt.Errorf("failed to deploy panel after %d attempts: %w", retryAttempts, deployErr)
}

func runCommand(cmdDir string, command string, retries int) (string, error) {
	var output string
	var err error
	for attempt := 1; attempt <= retries; attempt++ {
		cmd := exec.Command("sh", "-c", command)
		cmd.Dir = cmdDir
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		err = cmd.Run()
		stdOutput := stdoutBuf.String()
		stdErrOutput := stderrBuf.String()
		output = stdOutput + stdErrOutput

		if err == nil {
			return output, nil
		}

		if attempt < retries {
			printWarning(fmt.Sprintf("Command failed: %v. Retrying in %v...", err, retryDelay))
			time.Sleep(retryDelay)
		} else {
			return output, fmt.Errorf("command '%s' failed after %d attempts: %w", command, retries, err)
		}
	}

	return output, fmt.Errorf("command execution failed unexpectedly after %d retries: %w", retries, err)
}

func generateRandomString(charSet string, length int, isDomain bool) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomBytes := make([]byte, length)
	for i := range randomBytes {
		for {
			char := charSet[r.Intn(len(charSet))]

			if isDomain && length > 1 && (i == 0 || i == length-1) && char == '-' {
				continue
			}

			randomBytes[i] = char
			break
		}
	}
	return string(randomBytes)
}

func generateRandomDomain(subDomainLength int) string {
	return generateRandomString(domainCharset, subDomainLength, true)
}

func isWorkerAvailable(installDir, projectName, deployType string) (bool, error) {
	var command string
	if deployType == "1" {
		command = fmt.Sprintf("npx wrangler deployments list --name %s", projectName)
	} else {
		command = fmt.Sprintf("npx wrangler pages project list --project-name %s", projectName)
	}
	output, err := runCommand(installDir, command, 1)

	if err != nil {

		if strings.Contains(output, "No Projects found") || strings.Contains(output, "No deployments found") || strings.Contains(output, "could not find project") {
			return true, nil
		}

		return false, fmt.Errorf("checking availability failed: %w, output: %s", err, output)
	}

	return false, nil
}

func extractURL(output string) (string, error) {
	re := regexp.MustCompile(`https?://([a-zA-Z0-9-]+\.workers\.dev|[a-zA-Z0-9-]+\.pages\.dev)`)
	matches := re.FindAllString(output, -1)
	if len(matches) > 0 {

		return matches[len(matches)-1], nil
	}

	reFallback := regexp.MustCompile(`https?://[^\s]+`)
	matches = reFallback.FindAllString(output, -1)
	if len(matches) == 0 {
		return "", errors.New("no deployment URL found in output")
	}

	return matches[len(matches)-1], nil
}

func extractOAuthURL(output string) (string, error) {
	re := regexp.MustCompile(`https://dash\.cloudflare\.com/oauth2/auth\?[^ \n"'<>]+`)
	match := re.FindString(output)
	if match == "" {
		return "", errors.New("no Cloudflare OAuth URL found")
	}
	return match, nil
}

func openURL(url string) error {
	var cmdName string
	var cmdArgs []string

	if _, err := os.Stat("/data/data/com.termux"); err == nil {
		cmdName = "termux-open-url"
		cmdArgs = []string{url}
	} else {
		switch goos := runtime.GOOS; goos {
		case "linux":
			cmdName = "xdg-open"
			cmdArgs = []string{url}
		case "darwin":
			cmdName = "open"
			cmdArgs = []string{url}
		default:
			return fmt.Errorf("unsupported operating system for opening URL: %s", goos)
		}
	}

	cmd := exec.Command(cmdName, cmdArgs...)
	if cmdName == "termux-open-url" {
		cmd.Env = append(os.Environ(), "TERMUX_API_VERSION=0.50")
	}

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command '%s': %w", cmdName, err)
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func extractKvIDFromJSON(output string) (string, error) {
	var result struct {
		ID string `json:"id"`
	}
	err := json.Unmarshal([]byte(output), &result)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal KV create JSON output: %w, output: %s", err, output)
	}
	if result.ID == "" {
		return "", fmt.Errorf("no ID found in KV create JSON output: %s", output)
	}

	if matched, _ := regexp.MatchString(`^[a-fA-F0-9]{32}$`, result.ID); !matched {
		return "", fmt.Errorf("extracted KV ID '%s' does not look like a valid ID", result.ID)
	}
	return result.ID, nil
}

func downloadFileWithRetry(url, dest string, retries int) error {
	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		err := downloadFileAttempt(url, dest)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < retries {
			printWarning(fmt.Sprintf("Download attempt %d failed: %v. Retrying in %v...", attempt, err, retryDelay))
			time.Sleep(retryDelay)
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", retries, lastErr)
}

func downloadFileAttempt(url, dest string) (err error) {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %d from %s", resp.StatusCode, url)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating destination file %s: %w", dest, err)
	}
	defer func() {
		if cerr := out.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("closing destination file %s: %w", dest, cerr)
		}
	}()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("copying response body to %s: %w", dest, err)
	}

	return nil
}

func printTitle(message string) {
	fmt.Printf("\n%s %s\n", titlePrefix, message)
}

func printInfo(message string) {
	fmt.Printf("%s %s\n", infoPrefix, message)
}

func printWarning(message string) {
	fmt.Printf("%s %s\n", warnPrefix, message)
}

func printError(message string, err error) {
	fullMessage := message
	if err != nil {
		fullMessage += ": " + err.Error()
	}
	fmt.Printf("%s %s\n", errorPrefix, fullMessage)
}

func failMessage(message string, err error) {
	printError(message, err)
}

func successMessage(message string) {
	fmt.Printf("%s %s\n", successPrefix, message)
}

func printSuccessPanelInfo(panelURL string) {
	fmt.Printf("\n%s Panel installed successfully!\n%s Access it at: %s%s%s\n%s Copy this URL and open it in your browser to access the BPB Panel.\n", successPrefix, infoPrefix, blue, panelURL, reset, infoPrefix)
}