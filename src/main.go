package main

import (
	"bytes"
	"context"
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
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	wranglerVersionRequired = "4.12.0"
	workerFileURL         = "https://github.com/bia-pain-bache/BPB-Worker-Panel/releases/latest/download/worker.js"
	defaultProxyIP        = "bpb.yousef.isegaro.com"
	defaultFallbackDomain = "speed.cloudflare.com"
	nodeMajorVersionMin   = 18
	loginTimeout          = 60 * time.Second
	commandTimeout        = 120 * time.Second
	retryAttempts         = 3
	retryDelay            = 5 * time.Second
)

const (
	deployTypeWorkers = "1"
	deployTypePages   = "2"
)

var (
	kvID         string
	projectName  string
	customDomain string
	deployType   string
	generatedUUID string
	trPass       string
	proxyIP      string
	fallback     string
	subPath      string
	installDir   string
	wranglerTOMLPath string
	srcPath      string
	isTermux     bool
)

var (
	red     = "\033[0;31m"
	green   = "\033[0;32m"
	yellow  = "\033[0;33m"
	blue    = "\033[0;34m"
	cyan    = "\033[0;36m"
	reset   = "\033[0m"
	bold    = "\033[1m"

	titlePrefix   = bold + cyan + "◆" + reset
	infoPrefix    = bold + blue + "❯" + reset
	warnPrefix    = bold + yellow + "⚠" + reset
	errorPrefix   = bold + red + "✗" + reset
	successPrefix = bold + green + "✓" + reset
)

type WranglerConfig struct {
	Name               string            `toml:"name"`
	CompatibilityDate  string            `toml:"compatibility_date"`
	CompatibilityFlags []string          `toml:"compatibility_flags"`
	WorkersDev         *bool             `toml:"workers_dev,omitempty"`
	Main               string            `toml:"main,omitempty"`
	PagesBuildOutputDir string            `toml:"pages_build_output_dir,omitempty"`
	KVNamespaces       []KVNamespaceBind `toml:"kv_namespaces"`
	Vars               map[string]string `toml:"vars"`
}

type KVNamespaceBind struct {
	Binding string `toml:"binding"`
	ID      string `toml:"id"`
}

func main() {
	flag.StringVar(&deployType, "deploy", deployTypeWorkers, "Deployment type: 1 for Workers (default), 2 for Pages")
	flag.Parse()

	if deployType != deployTypeWorkers && deployType != deployTypePages {
		failMessage("Invalid deploy type. Use -deploy=1 for Workers or -deploy=2 for Pages.", nil)
		os.Exit(1)
	}

	if err := setupEnvironment(); err != nil {
		failMessage("Failed to set up environment", err)
		os.Exit(1)
	}

	fmt.Printf("\n%s Installing %sBPB Terminal Wizard%s...\n", titlePrefix, bold+blue, reset)

	if err := checkNode(); err != nil {
		failMessage(fmt.Sprintf("Node.js v%d+ is required.", nodeMajorVersionMin), err)
		os.Exit(1)
	}
	if err := checkAndInstallWrangler(); err != nil {
		failMessage("Failed to setup Wrangler", err)
		os.Exit(1)
	}
	successMessage("Dependencies verified/installed successfully!")

	fmt.Printf("\n%s Login to %sCloudflare%s...\n", titlePrefix, bold+yellow, reset)
	if err := cloudflareLogin(); err != nil {
		failMessage("Cloudflare login failed", err)
		os.Exit(1)
	}
	successMessage("Cloudflare logged in successfully!")

	fmt.Printf("\n%s Generating Configuration...\n", titlePrefix)
	if err := generateConfigValues(); err != nil {
		failMessage("Failed to generate configuration values", err)
		os.Exit(1)
	}

	fmt.Printf("\n%s Downloading %sworker script%s...\n", titlePrefix, bold+green, reset)
	workerFileName := "worker.js"
	if deployType == deployTypePages {
		workerFileName = "_worker.js"
	}
	workerPath := filepath.Join(srcPath, workerFileName)
	if err := downloadWorkerScript(workerPath); err != nil {
		failMessage("Failed to download worker script", err)
		os.Exit(1)
	}
	successMessage("Worker script downloaded successfully!")

	fmt.Printf("\n%s %sNote:%s This program creates a new KV namespace each time.\n   Check your Cloudflare account later to delete unused KV namespaces.\n", warnPrefix, bold, reset)
	fmt.Printf("\n%s Creating KV namespace...\n", titlePrefix)
	var err error
	kvID, err = createKVNamespaceWithRetry()
	if err != nil {
		failMessage("Failed to create KV namespace", err)
		os.Exit(1)
	}
	successMessage(fmt.Sprintf("KV namespace created successfully (ID: %s)", kvID))

	fmt.Printf("\n%s Building %swrangler.toml%s configuration...\n", titlePrefix, bold+green, reset)
	if err := buildWranglerConfig(wranglerTOMLPath); err != nil {
		failMessage("Error building wrangler.toml", err)
		os.Exit(1)
	}
	successMessage("wrangler.toml built successfully!")

	fmt.Printf("\n%s Deploying %sBPB Panel%s...\n", titlePrefix, bold+blue, reset)
	panelURL, err := deployPanel()
	if err != nil {
		failMessage("Panel deployment failed", err)
		os.Exit(1)
	}

	fmt.Printf("\n%s Panel deployed successfully!\n", successPrefix)
	if deployType == deployTypePages {
		fmt.Printf("%s It might take a few minutes for the Pages deployment to become fully active.\n", warnPrefix)
	}
	fmt.Printf("%s Access your panel at: %s%s%s\n", infoPrefix, blue, panelURL, reset)
	fmt.Printf("%s Trojan Password: %s%s%s\n", infoPrefix, cyan, trPass, reset)
	fmt.Printf("%s UUID: %s%s%s\n", infoPrefix, cyan, generatedUUID, reset)
	fmt.Printf("%s Subscription Path: %s%s%s\n", infoPrefix, cyan, subPath, reset)
}

func setupEnvironment() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	installDir = filepath.Join(homeDir, ".bpb-terminal-wizard")
	wranglerTOMLPath = filepath.Join(installDir, "wrangler.toml")
	srcPath = filepath.Join(installDir, "src")

	if _, err := os.Stat("/data/data/com.termux"); err == nil {
		isTermux = true
		fmt.Printf("%s Termux environment detected.\n", infoPrefix)
	}

	_ = os.Remove(filepath.Join(installDir, "wrangler.json"))
	_ = os.Remove(wranglerTOMLPath)
	_ = os.RemoveAll(srcPath)

	if err := os.MkdirAll(installDir, 0750); err != nil {
		return fmt.Errorf("creating install directory '%s': %w", installDir, err)
	}
	if err := os.MkdirAll(srcPath, 0750); err != nil {
		return fmt.Errorf("creating source directory '%s': %w", srcPath, err)
	}
	return nil
}

func checkNode() error {
	fmt.Printf("%s Checking Node.js version...\n", infoPrefix)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "-v")
	outputBytes, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("could not execute 'node -v': %w. Ensure Node.js is installed and in PATH", err)
	}
	version := strings.TrimSpace(string(outputBytes))
	re := regexp.MustCompile(`^v(\d+)`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 2 {
		return fmt.Errorf("could not parse node version string: %s", version)
	}
	majorVersionStr := matches[1]
	var majorVersion int
	_, err = fmt.Sscan(majorVersionStr, &majorVersion)
	if err != nil {
		return fmt.Errorf("could not parse node major version '%s': %w", majorVersionStr, err)
	}

	if majorVersion < nodeMajorVersionMin {
		return fmt.Errorf("node.js version %s is too old; version %d or higher required", version, nodeMajorVersionMin)
	}
	successMessage(fmt.Sprintf("Node.js version %s is compatible.", version))
	return nil
}

func runCommand(ctx context.Context, cmdDir string, command ...string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("no command provided")
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	if cmdDir != "" {
		cmd.Dir = cmdDir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	fmt.Printf("%s Executing: %s %s\n", infoPrefix, command[0], strings.Join(command[1:], " "))

	err := cmd.Run()
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()
	output := stdoutStr + stderrStr

	if err != nil {
		return output, fmt.Errorf("command failed: %w. Stderr: %s", err, strings.TrimSpace(stderrStr))
	}

	return output, nil
}


func checkAndInstallWrangler() error {
	fmt.Printf("%s Checking Wrangler version...\n", infoPrefix)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

    wranglerCmd := "wrangler"
    if _, err := exec.LookPath(wranglerCmd); err != nil {
        fmt.Printf("%s 'wrangler' not found in PATH, trying 'npx wrangler'...\n", infoPrefix)
        wranglerCmd = "npx"
    }

    args := []string{"--version"}
    if wranglerCmd == "npx" {
        args = append([]string{"wrangler"}, args...)
    }

	output, err := runCommand(ctx, installDir, append([]string{wranglerCmd}, args...)...)
	if err == nil && strings.Contains(output, wranglerVersionRequired) {
		successMessage(fmt.Sprintf("Wrangler version %s found.", wranglerVersionRequired))
		disableTelemetryCtx, disableCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer disableCancel()
        telemetryArgs := []string{"telemetry", "disable"}
        if wranglerCmd == "npx" {
             telemetryArgs = append([]string{"wrangler"}, telemetryArgs...)
        }
		runCommand(disableTelemetryCtx, installDir, append([]string{wranglerCmd}, telemetryArgs...)...)
		return nil
	}

	fmt.Printf("%s Wrangler not found or wrong version. Attempting installation...\n", warnPrefix)

	installCtx, installCancel := context.WithTimeout(context.Background(), commandTimeout)
	defer installCancel()

    fmt.Printf("%s Cleaning npm cache...\n", infoPrefix)
	runCommand(installCtx, installDir, "npm", "cache", "clean", "--force")

    fmt.Printf("%s Attempting to uninstall previous Wrangler versions...\n", infoPrefix)
    runCommand(installCtx, installDir, "npm", "uninstall", "-g", "wrangler")


	fmt.Printf("%s Installing Wrangler v%s globally...\n", infoPrefix, wranglerVersionRequired)
	_, err = runCommand(installCtx, installDir, "npm", "install", "-g", fmt.Sprintf("wrangler@%s", wranglerVersionRequired))
	if err != nil {
		return fmt.Errorf("npm install failed: %w. Check npm logs", err)
	}

	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer verifyCancel()

    wranglerCmd = "wrangler"
    if _, err := exec.LookPath(wranglerCmd); err != nil {
        wranglerCmd = "npx"
    }
    args = []string{"--version"}
    if wranglerCmd == "npx" {
        args = append([]string{"wrangler"}, args...)
    }

	output, err = runCommand(verifyCtx, installDir, append([]string{wranglerCmd}, args...)...)
	if err != nil {
		return fmt.Errorf("failed to run Wrangler after installation: %w", err)
	}
	if !strings.Contains(output, wranglerVersionRequired) {
		return fmt.Errorf("installed Wrangler version mismatch. Expected %s, got output: %s", wranglerVersionRequired, output)
	}

	successMessage(fmt.Sprintf("Wrangler v%s installed successfully.", wranglerVersionRequired))
	disableTelemetryCtx, disableCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer disableCancel()
    telemetryArgs := []string{"telemetry", "disable"}
    if wranglerCmd == "npx" {
         telemetryArgs = append([]string{"wrangler"}, telemetryArgs...)
    }
	runCommand(disableTelemetryCtx, installDir, append([]string{wranglerCmd}, telemetryArgs...)...)
	return nil
}

func cloudflareLogin() error {
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		fmt.Printf("%s Starting Cloudflare login (attempt %d/%d)...\n", infoPrefix, attempt, retryAttempts)
		cmd := exec.Command("npx", "wrangler", "login")
		cmd.Dir = installDir
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		cmd.Stdin = os.Stdin

		if err := cmd.Start(); err != nil {
			fmt.Printf("%s Error starting wrangler login: %v\n", errorPrefix, err)
			time.Sleep(retryDelay)
			continue
		}

		oauthURLChan := make(chan string)
		errChan := make(chan error, 1)
		doneChan := make(chan bool)

		go func() {
			defer close(doneChan)
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					url, extracted := extractOAuthURL(stdoutBuf.String())
					if extracted {
						oauthURLChan <- url
						return
					}
				case <-time.After(loginTimeout):
					errChan <- fmt.Errorf("timeout waiting for OAuth URL from wrangler")
					return
				}
			}
		}()

        var loginErr error
		select {
		case url := <-oauthURLChan:
			fmt.Printf("%s Login URL detected. Opening browser...\n", infoPrefix)
			if err := openURL(url); err != nil {
				fmt.Printf("%s Could not open browser automatically.\nPlease open this URL manually: %s%s%s\n", warnPrefix, blue, url, reset)
			} else {
				fmt.Printf("%s Browser opened. Please authenticate in the browser.\n", infoPrefix)
			}
            loginErr = cmd.Wait()

		case err := <-errChan:
			_ = cmd.Process.Kill()
			cmd.Wait()
			fmt.Printf("%s Error during login URL detection: %v\n", errorPrefix, err)
            fmt.Printf("%s Wrangler output:\n%s\n%s\n", warnPrefix, stdoutBuf.String(), stderrBuf.String())
			time.Sleep(retryDelay)
			continue

		case <-time.After(loginTimeout + 5*time.Second):
            _ = cmd.Process.Kill()
            cmd.Wait()
			fmt.Printf("%s Overall timeout waiting for Cloudflare login process to complete.\n", errorPrefix)
            fmt.Printf("%s Wrangler output:\n%s\n%s\n", warnPrefix, stdoutBuf.String(), stderrBuf.String())
            time.Sleep(retryDelay)
            continue
		}

        <-doneChan

		if loginErr != nil {
			fmt.Printf("%s 'wrangler login' command failed: %v\n", errorPrefix, loginErr)
            fmt.Printf("%s Wrangler output:\n%s\n%s\n", warnPrefix, stdoutBuf.String(), stderrBuf.String())
			time.Sleep(retryDelay)
			continue
		}

        fmt.Printf("%s Verifying login status...\n", infoPrefix)
        verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 20*time.Second)
        _, verifyErr := runCommand(verifyCtx, installDir, "npx", "wrangler", "whoami")
        verifyCancel()
        if verifyErr != nil {
             fmt.Printf("%s Login verification failed: %v. Retrying login...\n", warnPrefix, verifyErr)
             time.Sleep(retryDelay)
             continue
        }
        fmt.Printf("%s Login verified.\n", successPrefix)
		return nil
	}
	return fmt.Errorf("failed to login to Cloudflare after %d attempts", retryAttempts)
}


func extractOAuthURL(output string) (string, bool) {
	re := regexp.MustCompile(`https://dash\.cloudflare\.com/oauth2/auth\?[^\s"']+`)
	match := re.FindString(output)
	return match, match != ""
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
        if isTermux {
             fmt.Printf("%s Using termux-open-url...\n", infoPrefix)
			 cmd = exec.Command("termux-open-url", url)
			 cmd.Env = append(os.Environ(), "TERMUX_API_VERSION=0.50")
        } else {
            opener := "xdg-open"
            if _, err := exec.LookPath(opener); err != nil {
                opener = "gnome-open"
                 if _, err := exec.LookPath(opener); err != nil {
                      opener = "kde-open"
                      if _, err := exec.LookPath(opener); err != nil {
                           return fmt.Errorf("cannot find a suitable URL opener (xdg-open, gnome-open, kde-open)")
                      }
                 }
            }
             fmt.Printf("%s Using %s...\n", infoPrefix, opener)
			 cmd = exec.Command(opener, url)
        }
	case "darwin":
        fmt.Printf("%s Using open (macOS)...\n", infoPrefix)
		cmd = exec.Command("open", url)
	case "windows":
        fmt.Printf("%s Using start (Windows)...\n", infoPrefix)
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported operating system for opening URL: %s", runtime.GOOS)
	}

    cmd.Stdout = nil
    cmd.Stderr = nil
    err := cmd.Start()
    if err != nil {
        return fmt.Errorf("failed to start URL opener '%s': %w", cmd.Path, err)
    }
	return nil
}


func generateConfigValues() error {
	fmt.Printf("%s Selected deployment type: %s%s%s\n", infoPrefix, bold+green, map[string]string{deployTypeWorkers: "Cloudflare Workers", deployTypePages: "Cloudflare Pages"}[deployType], reset)
	if deployType == deployTypePages {
		fmt.Printf("%s Note: With %sPages%s deployment, modifying settings later via the Cloudflare dashboard might be limited.\n", warnPrefix, bold+green, reset)
	}

	for {
		projectName = generateRandomString("abcdefghijklmnopqrstuvwxyz0123456789", 20, true)
		fmt.Printf("\n%s Generated project name: %s%s%s\n", infoPrefix, cyan, projectName, reset)

		fmt.Printf("%s Checking availability of '%s'...\n", infoPrefix, projectName)
		available, err := isProjectNameAvailable(projectName, deployType)
		if err != nil {
			fmt.Printf("%s Could not check project name availability: %v. Retrying with a new name...\n", warnPrefix, err)
			continue
		}
		if available {
			successMessage(fmt.Sprintf("Project name '%s' is available!", projectName))
			break
		} else {
			fmt.Printf("%s Project name '%s' is likely already taken. Generating a new one...\n", warnPrefix, projectName)
		}
	}

	generatedUUID = uuid.NewString()
	fmt.Printf("\n%s Generated %sUUID%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, generatedUUID, reset)

	trPass = generateRandomString("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+[]{}|;:',.<>?", 16, false)
	fmt.Printf("\n%s Generated %sTrojan password%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, trPass, reset)

	proxyIP = defaultProxyIP
	fmt.Printf("\n%s Using default %sProxy IP%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, proxyIP, reset)

	fallback = defaultFallbackDomain
	fmt.Printf("\n%s Using default %sFallback domain%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, fallback, reset)

	subPath = generateRandomString("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", 12, false)
	fmt.Printf("\n%s Generated %sSubscription path%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, subPath, reset)

	return nil
}


func isProjectNameAvailable(name, deployType string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmdArgs []string
	if deployType == deployTypeWorkers {
		cmdArgs = []string{"npx", "wrangler", "deployments", "list", "--name", name}
	} else {
		cmdArgs = []string{"npx", "wrangler", "pages", "project", "list"}
	}

	output, err := runCommand(ctx, installDir, cmdArgs...)

	if deployType == deployTypeWorkers {
		if err != nil {
			if strings.Contains(output, "No deployments found") || strings.Contains(strings.ToLower(output), "not found") {
				return true, nil
			}
			return false, fmt.Errorf("checking worker availability failed: %w, Output: %s", err, output)
		}
		return false, nil
	} else {
		if err != nil {
			return false, fmt.Errorf("checking pages project list failed: %w, Output: %s", err, output)
		}
		return !strings.Contains(output, name), nil
	}
}


func downloadWorkerScript(destPath string) error {
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		fmt.Printf("%s Attempt %d/%d: Downloading %s from %s...\n", infoPrefix, attempt, retryAttempts, filepath.Base(destPath), workerFileURL)

        req, err := http.NewRequestWithContext(context.Background(), "GET", workerFileURL, nil)
        if err != nil {
            return fmt.Errorf("creating request: %w", err)
        }

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("%s Download error (attempt %d): %v\n", warnPrefix, attempt, err)
			if attempt < retryAttempts {
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("making GET request after %d attempts: %w", attempt, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			errMsg := fmt.Sprintf("bad status: %s. Body: %s", resp.Status, string(bodyBytes))
			fmt.Printf("%s Download failed (attempt %d): %s\n", warnPrefix, attempt, errMsg)
			if attempt < retryAttempts {
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("failed to download file '%s' after %d attempts: %s", workerFileURL, attempt, resp.Status)
		}

		out, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("creating destination file '%s': %w", destPath, err)
		}

		_, err = io.Copy(out, resp.Body)
		out.Close()
		if err != nil {
			return fmt.Errorf("writing downloaded content to '%s': %w", destPath, err)
		}

		return nil
	}
	return fmt.Errorf("download failed after %d attempts", retryAttempts)
}


func createKVNamespaceWithRetry() (string, error) {
	var lastErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		kvName := fmt.Sprintf("bpb-panel-kv-%s", generateRandomString("abcdefghijklmnopqrstuvwxyz0123456789", 8, false))
		fmt.Printf("%s Attempt %d/%d: Creating KV namespace '%s'...\n", infoPrefix, attempt, retryAttempts, kvName)

		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		output, err := runCommand(ctx, installDir, "npx", "wrangler", "kv", "namespace", "create", kvName)
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("wrangler command failed: %w, Output: %s", err, output)
			fmt.Printf("%s KV creation failed (attempt %d): %v\n", warnPrefix, attempt, lastErr)
			if strings.Contains(strings.ToLower(output), "fetch failed") || strings.Contains(strings.ToLower(output), "timed out") {
                if attempt < retryAttempts {
				    fmt.Printf("%s Retrying after %v...\n", infoPrefix, retryDelay)
				    time.Sleep(retryDelay)
				    continue
                }
			}
            continue
		}

		id, extractErr := extractKvID(output)
		if extractErr != nil {
			lastErr = fmt.Errorf("could not extract KV ID from output: %w, Output: %s", extractErr, output)
			fmt.Printf("%s KV ID extraction failed (attempt %d): %v\n", warnPrefix, attempt, lastErr)
            if attempt < retryAttempts {
                 time.Sleep(retryDelay)
                 continue
            }
            return "", lastErr
		}

		return id, nil
	}
	return "", fmt.Errorf("failed to create KV namespace after %d attempts: %w", retryAttempts, lastErr)
}

func extractKvID(output string) (string, error) {
	reJSON := regexp.MustCompile(`(?s)\{\s*.*"id":\s*"([^"]+)".*\s*\}`)
	matchesJSON := reJSON.FindStringSubmatch(output)
	if len(matchesJSON) >= 2 {
		return matchesJSON[1], nil
	}

	reSimple := regexp.MustCompile(`id:\s*([a-fA-F0-9]+)`)
	matchesSimple := reSimple.FindStringSubmatch(output)
	if len(matchesSimple) >= 2 {
		return matchesSimple[1], nil
	}

	reBind := regexp.MustCompile(`binding\s*=\s*".*"\s*id\s*=\s*"([^"]+)"`)
	matchesBind := reBind.FindStringSubmatch(output)
	if len(matchesBind) >= 2 {
		return matchesBind[1], nil
	}


	return "", fmt.Errorf("no valid KV ID found in output")
}


func buildWranglerConfig(filePath string) error {
	workersDev := true
	config := WranglerConfig{
		Name:              projectName,
		CompatibilityDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		CompatibilityFlags: []string{"nodejs_compat"},
		KVNamespaces: []KVNamespaceBind{
			{
				Binding: "KV",
				ID:      kvID,
			},
		},
		Vars: map[string]string{
			"UUID":     generatedUUID,
			"TR_PASS":  trPass,
			"PROXY_IP": proxyIP,
			"FALLBACK": fallback,
			"SUB_PATH": subPath,
		},
	}

	if deployType == deployTypeWorkers {
		config.Main = filepath.Join(".", filepath.Base(srcPath), "worker.js")
		config.WorkersDev = &workersDev
	} else {
		config.PagesBuildOutputDir = filepath.Join(".", filepath.Base(srcPath))
	}


	var tomlContent strings.Builder
	tomlContent.WriteString(fmt.Sprintf("name = \"%s\"\n", config.Name))
	tomlContent.WriteString(fmt.Sprintf("compatibility_date = \"%s\"\n", config.CompatibilityDate))
	tomlContent.WriteString("compatibility_flags = [\"nodejs_compat\"]\n\n")

	if config.Main != "" {
		tomlContent.WriteString(fmt.Sprintf("main = \"%s\"\n", config.Main))
	}
	if config.WorkersDev != nil && *config.WorkersDev {
        tomlContent.WriteString("workers_dev = true\n")
    }
	if config.PagesBuildOutputDir != "" {
		tomlContent.WriteString(fmt.Sprintf("pages_build_output_dir = \"%s\"\n", config.PagesBuildOutputDir))
	}

	tomlContent.WriteString("\n[[kv_namespaces]]\n")
	tomlContent.WriteString(fmt.Sprintf("binding = \"%s\"\n", config.KVNamespaces[0].Binding))
	tomlContent.WriteString(fmt.Sprintf("id = \"%s\"\n\n", config.KVNamespaces[0].ID))

	tomlContent.WriteString("[vars]\n")
	tomlContent.WriteString(fmt.Sprintf("UUID = \"%s\"\n", config.Vars["UUID"]))
	tomlContent.WriteString(fmt.Sprintf("TR_PASS = \"%s\"\n", config.Vars["TR_PASS"]))
	tomlContent.WriteString(fmt.Sprintf("PROXY_IP = \"%s\"\n", config.Vars["PROXY_IP"]))
	tomlContent.WriteString(fmt.Sprintf("FALLBACK = \"%s\"\n", config.Vars["FALLBACK"]))
	tomlContent.WriteString(fmt.Sprintf("SUB_PATH = \"%s\"\n", config.Vars["SUB_PATH"]))

	if err := os.WriteFile(filePath, []byte(tomlContent.String()), 0644); err != nil {
		return fmt.Errorf("writing wrangler.toml to '%s': %w", filePath, err)
	}

	fmt.Printf("%s Generated wrangler.toml:\n---\n%s\n---\n", infoPrefix, tomlContent.String())

	return nil
}

func deployPanel() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout*2)
	defer cancel()
	var panelURL string

	for attempt := 1; attempt <= retryAttempts; attempt++ {
		fmt.Printf("%s Deploy attempt %d/%d...\n", infoPrefix, attempt, retryAttempts)
		var output string
		var err error

		if deployType == deployTypeWorkers {
            workerScriptPath := filepath.Join(srcPath, "worker.js")
			output, err = runCommand(ctx, installDir, "npx", "wrangler", "deploy", workerScriptPath)
			if err == nil {
                url, extractErr := extractWorkerURL(output)
                 if extractErr != nil {
                      return "", fmt.Errorf("deployment succeeded but failed to extract URL: %w, Output: %s", extractErr, output)
                 }
				panelURL = url + "/panel"
				return panelURL, nil
			}
		} else {
			fmt.Printf("%s Creating/Verifying Pages project '%s'...\n", infoPrefix, projectName)
			createArgs := []string{"npx", "wrangler", "pages", "project", "create", projectName, "--production-branch", "main"}
			_, createErr := runCommand(ctx, installDir, createArgs...)
			if createErr != nil {
                 if !strings.Contains(strings.ToLower(createErr.Error()), "project already exists") {
                    err = fmt.Errorf("creating pages project failed: %w", createErr)
                    fmt.Printf("%s Warning: %v. Proceeding to deploy step...\n", warnPrefix, err)
                 } else {
                      fmt.Printf("%s Pages project '%s' already exists.\n", infoPrefix, projectName)
                 }
			} else {
                 fmt.Printf("%s Pages project '%s' created/verified.\n", successPrefix, projectName)
            }


			fmt.Printf("%s Deploying to Pages project '%s'...\n", infoPrefix, projectName)
			deployArgs := []string{"npx", "wrangler", "pages", "deploy", filepath.Base(srcPath), "--project-name", projectName, "--branch", "main", "--commit-dirty=true"}
			output, err = runCommand(ctx, installDir, deployArgs...)
			if err == nil {
				panelURL = fmt.Sprintf("https://%s.pages.dev/panel", projectName)
				return panelURL, nil
			}
		}

		fmt.Printf("%s Deployment failed (attempt %d): %v\n", errorPrefix, attempt, err)
        fmt.Printf("%s Output:\n%s\n", warnPrefix, output)

		if attempt < retryAttempts {
             retryable := false
             lowerOutput := strings.ToLower(output)
             lowerError := ""
             if err != nil { lowerError = strings.ToLower(err.Error()) }

             if strings.Contains(lowerOutput, "timed out") || strings.Contains(lowerError, "timed out") ||
                strings.Contains(lowerOutput, "try again") || strings.Contains(lowerError, "try again") ||
                strings.Contains(lowerOutput, "503") || strings.Contains(lowerError, "503") ||
                strings.Contains(lowerOutput, "rate limit") || strings.Contains(lowerError, "rate limit") {
                 retryable = true
             }

             if retryable {
			    fmt.Printf("%s Retrying after %v...\n", infoPrefix, retryDelay)
			    time.Sleep(retryDelay)
			    continue
             } else {
                  fmt.Printf("%s Error seems non-retryable. Aborting.\n", errorPrefix)
                  return "", fmt.Errorf("non-retryable deployment error after attempt %d: %w", attempt, err)
             }
		}
         return "", fmt.Errorf("deployment failed after %d attempts: %w", attempt, err)
	}

    return "", fmt.Errorf("deployment failed after %d attempts", retryAttempts)
}

func extractWorkerURL(output string) (string, error) {
	re := regexp.MustCompile(`https://[a-zA-Z0-9-]+[.][a-zA-Z0-9-]+[.](workers\.dev|pages\.dev)`)
	matches := re.FindAllString(output, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	return "", fmt.Errorf("no deployed worker URL found in wrangler output")
}

func generateRandomString(charSet string, length int, isDomain bool) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]byte, length)
	firstCharIndex := 0
	lastCharIndex := length - 1

	for i := 0; i < length; i++ {
		for {
			char := charSet[r.Intn(len(charSet))]
			valid := true

			if isDomain {
				if (i == firstCharIndex || i == lastCharIndex) && char == '-' {
					valid = false
				}
			} else {
				if i == firstCharIndex && char >= '0' && char <= '9' {
					valid = false
				}
			}

			if valid {
				result[i] = char
				break
			}
		}
	}
	return string(result)
}

func failMessage(message string, err error) {
	errMsg := message
	if err != nil {
		errMsg += ": " + err.Error()
	}
	fmt.Printf("\n%s %s\n\n", errorPrefix, errMsg)
}

func successMessage(message string) {
	fmt.Printf("%s %s\n", successPrefix, message)
}