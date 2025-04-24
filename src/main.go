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
	"strings"
	"time"

	"github.com/google/uuid"
)

type KVNamespace struct {
	Title string `json:"title"`
	ID    string `json:"id"`
}

var (
	kvID         string
	projectName  string
	customDomain string
	deployType   string
	UUID         string
	TR_PASS      string
	PROXY_IP     string
	FALLBACK     string
	SUB_PATH     string
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

func main() {
	var deployFlag string
	flag.StringVar(&deployFlag, "deploy", "1", "Deployment type: 1 for Workers, 2 for Pages")
	flag.Parse()

	if deployFlag != "1" && deployFlag != "2" {
		failMessage("Invalid deploy type. Use -deploy=1 for Workers or -deploy=2 for Pages.", nil)
		return
	}
	deployType = deployFlag

	homeDir, err := os.UserHomeDir()
	if err != nil {
		failMessage("Error getting home directory", err)
		return
	}
	installDir := filepath.Join(homeDir, ".bpb-terminal-wizard")
	wranglerConfigPath := filepath.Join(installDir, "wrangler.json")
	workerURL := "https://github.com/bia-pain-bache/BPB-Worker-Panel/releases/latest/download/worker.js"
	srsPath := filepath.Join(installDir, "src")

	if _, err := os.Stat(wranglerConfigPath); !errors.Is(err, os.ErrNotExist) {
		fmt.Printf("%s Cleaning up old worker config...\n", infoPrefix)
		if err := os.Remove(wranglerConfigPath); err != nil {
			warnMessage("Error deleting old worker config. Continuing anyway.")
		}
	}

	if _, err := os.Stat(srsPath); !errors.Is(err, os.ErrNotExist) {
		fmt.Printf("%s Cleaning up old worker.js file...\n", infoPrefix)
		if err := os.RemoveAll(srsPath); err != nil {
			warnMessage("Error deleting old worker.js directory. Continuing anyway.")
		}
	}


	if err := os.MkdirAll(installDir, 0750); err != nil {
		failMessage("Error creating install directory", err)
		return
	}

	fmt.Printf("\n%s Installing %sBPB Terminal Wizard%s...\n", titlePrefix, bold+blue, reset)

	successMessage("BPB Terminal Wizard dependencies are ready!")


	fmt.Printf("\n%s Login %sCloudflare%s...\n", titlePrefix, bold+yellow, reset)
	for {
		cmd := exec.Command("sh", "-c", "npx wrangler login")
		cmd.Dir = installDir
		var stdoutBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		fmt.Printf("%s Running 'npx wrangler login'. This may open a browser window.\n", infoPrefix)

		if err := cmd.Start(); err != nil {
			failMessage("Error starting Cloudflare login process", err)
			continue
		}

		timeout := time.After(60 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		var oauthURL string
		urlFound := false
		for !urlFound {
			select {
			case <-timeout:
				fmt.Printf("%s Debug: Wrangler output: %s\n", infoPrefix, stdoutBuf.String())
				failMessage("Timeout waiting for Cloudflare login URL. Please try again.", nil)
				return
			case <-ticker.C:
				oauthURL, err = extractOAuthURL(stdoutBuf.String())
				if err == nil {
					urlFound = true
					if err := openURL(oauthURL); err != nil {
						fmt.Printf("%s Could not open browser automatically.\nPlease open this URL manually: %s%s%s\n", warnPrefix, blue, oauthURL, reset)
					} else {
						fmt.Printf("%s Browser opened with URL: %s%s%s\n", infoPrefix, blue, oauthURL, reset)
					}
					time.Sleep(5 * time.Second)
				}
			}
		}

		if err := cmd.Wait(); err != nil {
			if strings.Contains(err.Error(), "user aborted") || strings.Contains(err.Error(), "canceled") {
				failMessage("Cloudflare login was cancelled by the user.", nil)
				return
			}
			failMessage("Error during Cloudflare login process", err)
			continue
		}

		fmt.Printf("%s Attempting to disable Wrangler telemetry...\n", infoPrefix)
		if _, err := runCommand(installDir, "npx wrangler telemetry disable"); err != nil {
			warnMessage("Could not disable telemetry. Continuing anyway.")
		} else {
			successMessage("Wrangler telemetry disabled.")
		}

		successMessage("Cloudflare logged in successfully!")
		break
	}


	fmt.Printf("\n%s Get Worker settings...\n", titlePrefix)

	fmt.Printf("\n%s Using deployment type: %s%s%s\n", infoPrefix, bold+green, map[string]string{"1": "Workers", "2": "Pages"}[deployType], reset)
	if deployType == "2" {
		fmt.Printf("%s With %sPages%s, you cannot modify settings later from Cloudflare dashboard.\n", warnPrefix, bold+green, reset)
		fmt.Printf("%s With %sPages%s, it may take up to 5 minutes to access the panel.\n", warnPrefix, bold+green, reset)
	}

	for {
		projectName = generateRandomDomain(32)
		fmt.Printf("\n%s Generated worker name (%sSubdomain%s): %s%s%s\n", infoPrefix, bold+green, reset, cyan, projectName, reset)
		successMessage("Using generated worker name.")

		fmt.Printf("\n%s Checking domain availability...\n", titlePrefix)
		if isWorkerAvailable(installDir, projectName, deployType) {
			warnMessage(fmt.Sprintf("Project name %s is not available. Generating a new one.", projectName))
			continue
		}
		successMessage("Available!")
		break
	}


	UUID = uuid.NewString()
	fmt.Printf("\n%s Generated %sUUID%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, UUID, reset)
	successMessage("Using generated UUID.")

	TR_PASS = generateTrPassword(12)
	fmt.Printf("\n%s Generated %sTrojan password%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, TR_PASS, reset)
	successMessage("Using generated Trojan password.")

	PROXY_IP = "bpb.yousef.isegaro.com"
	fmt.Printf("\n%s Default %sProxy IP%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, PROXY_IP, reset)
	successMessage("Using default Proxy IP.")

	FALLBACK = "speed.cloudflare.com"
	fmt.Printf("\n%s Default %sFallback domain%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, FALLBACK, reset)
	successMessage("Using default Fallback domain.")

	SUB_PATH = generateSubURIPath(16)
	fmt.Printf("\n%s Generated %sSubscription path%s: %s%s%s\n", infoPrefix, bold+green, reset, cyan, SUB_PATH, reset)
	successMessage("Using generated Subscription path.")


	fmt.Printf("\n%s Downloading %sworker.js%s...\n", titlePrefix, bold+green, reset)
	if err := os.MkdirAll(srsPath, 0750); err != nil {
		failMessage("Could not create src directory", err)
		return
	}

	var workerPath = filepath.Join(srsPath, "worker.js")
	if deployType == "2" {
		workerPath = filepath.Join(srsPath, "_worker.js")
	}
	for attempt := 1; attempt <= 3; attempt++ {
		fmt.Printf("%s Attempt %d to download worker.js...\n", infoPrefix, attempt)
		if err := downloadFile(workerURL, workerPath); err != nil {
			if attempt < 3 {
				warnMessage(fmt.Sprintf("Error downloading worker.js on attempt %d: %v. Retrying...", attempt, err))
				time.Sleep(5 * time.Second)
				continue
			}
			failMessage("Failed to download worker.js after multiple attempts", err)
			return
		}
		successMessage("Worker downloaded successfully!")
		break
	}


	fmt.Printf("\n%s This program creates a new KV namespace each time it runs.\n   Check your Cloudflare account and delete unused KV namespaces to avoid limits.\n", warnPrefix)
	fmt.Printf("\n%s Creating KV namespace...\n", titlePrefix)
	for attempt := 1; attempt <= 5; attempt++ {
		kvName := fmt.Sprintf("panel_kv_%s", generateRandomString("abcdefghijklmnopqrstuvwxyz0123456789", 8, false))
		fmt.Printf("%s Attempt %d to create KV namespace %s...\n", infoPrefix, attempt, kvName)
		output, err := runCommand(installDir, fmt.Sprintf("npx wrangler kv namespace create %s", kvName))
		if err != nil {
			message := fmt.Sprintf("Error creating KV on attempt %d! Output: %s. Check logs at ~/.config/.wrangler/logs/ for details.", attempt, output)
			if strings.Contains(output, "fetch failed") && attempt < 5 {
				warnMessage(message + " Retrying after 5 seconds...")
				time.Sleep(5 * time.Second)
				continue
			}
			failMessage(message, err)
			if attempt == 5 {
				return
			}
			continue
		}

		id, err := extractKvID(output)
		if err != nil {
			message := fmt.Sprintf("Error getting KV ID from output: %s. Check logs at ~/.config/.wrangler/logs/ for details.", output)
			failMessage(message, err)
			return
		}

		kvID = id
		successMessage("KV created successfully!")
		break
	}

	if kvID == "" {
		failMessage("Failed to create KV namespace after multiple attempts.", nil)
		return
	}


	fmt.Printf("\n%s Building panel configuration...\n", titlePrefix)
	if err := buildWranglerConfig(wranglerConfigPath); err != nil {
		failMessage("Error building Wrangler configuration", err)
		return
	}
	successMessage("Panel configuration built successfully!")

	var panelURL string
	for attempt := 1; attempt <= 3; attempt++ {
		fmt.Printf("\n%s Deploying %sBPB Panel%s (Attempt %d)...\n", titlePrefix, bold+blue, reset, attempt)
		var output string
		var err error

		if deployType == "1" {
			output, err = runCommand(installDir, "npx wrangler deploy ./src/worker.js")
			if err != nil {
				if attempt < 3 {
					warnMessage(fmt.Sprintf("Error deploying Panel on attempt %d: %s. Retrying...", attempt, output))
					time.Sleep(10 * time.Second)
					continue
				}
				failMessage(fmt.Sprintf("Error deploying Panel after multiple attempts! Output: %s", output), err)
				return
			}

			successMessage("Panel deployed successfully!")
			url, extractErr := extractURL(output)
			if extractErr != nil {
				failMessage("Error getting URL from deployment output", extractErr)
				return
			}
			panelURL = url + "/panel"
			break
		}

		if deployType == "2" {
			if attempt == 1 {
				fmt.Printf("%s Creating Pages project %s...\n", infoPrefix, projectName)
				createOutput, createErr := runCommand(installDir, fmt.Sprintf("npx wrangler pages project create %s --production-branch production", projectName))
				if createErr != nil {
					if strings.Contains(createOutput, "already exists") {
						warnMessage(fmt.Sprintf("Pages project %s already exists. Skipping creation.", projectName))
					} else {
						failMessage(fmt.Sprintf("Error creating Pages project! Output: %s", createOutput), createErr)
						if attempt < 3 {
							warnMessage("Retrying deployment...")
							time.Sleep(5 * time.Second)
							continue
						}
						return
					}
				} else {
					successMessage(fmt.Sprintf("Pages project %s created successfully!", projectName))
				}
			}

			fmt.Printf("%s Deploying Pages project...\n", infoPrefix)
			output, err = runCommand(installDir, "npx wrangler pages deploy ./src --commit-dirty true --branch production")
			if err != nil {
				if attempt < 3 {
					warnMessage(fmt.Sprintf("Error deploying Pages on attempt %d: %s. Retrying...", attempt, output))
					time.Sleep(10 * time.Second)
					continue
				}
				failMessage(fmt.Sprintf("Error deploying Pages after multiple attempts! Output: %s", output), err)
				return
			}
			successMessage("Panel deployed successfully!")
			panelURL = "https://" + projectName + ".pages.dev/panel"
			break
		}
	}

	if panelURL == "" {
		failMessage("Panel deployment failed after multiple attempts.", nil)
		return
	}


	fmt.Printf("\n%s Panel installed successfully!\n%s Access it at: %s%s%s\n", successPrefix, infoPrefix, blue, panelURL, reset)
}

func checkNode() error {
	output, err := exec.Command("node", "-v").Output()
	if err != nil {
		return fmt.Errorf("Node.js is not installed or not working: %v", err)
	}
	version := strings.TrimSpace(string(output))
	if !strings.HasPrefix(version, "v18") && !strings.HasPrefix(version, "v20") && !strings.HasPrefix(version, "v22") {
		return fmt.Errorf("Node.js version %s is outdated; please upgrade to 18.x or higher", version)
	}
	return nil
}

func runCommand(cmdDir string, command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = cmdDir
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	output := stdoutBuf.String() + stderrBuf.String()
	if err != nil {
		return output, fmt.Errorf("command failed: %v, output: %s", err, output)
	}
	return output, nil
}

func isValidDomain(domain string) bool {
	re, err := regexp.Compile(`^([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$`)
	if err != nil {
		return false
	}
	return re.MatchString(domain)
}

func generateRandomString(charSet string, length int, isDomain bool) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomBytes := make([]byte, length)
	for i := range randomBytes {
		for {
			char := charSet[r.Intn(len(charSet))]
			if isDomain && (i == 0 || i == length-1) && char == byte('-') {
				continue
			}
			randomBytes[i] = char
			break
		}
	}
	return string(randomBytes)
}

func generateRandomDomain(subDomainLength int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789-"
	return generateRandomString(charset, subDomainLength, true)
}

func generateTrPassword(passwordLength int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+[]{}|;:',.<>?"
	return generateRandomString(charset, passwordLength, false)
}

func generateSubURIPath(uriLength int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@$&*_-+;:,."
	return generateRandomString(charset, uriLength, false)
}

func isWorkerAvailable(installDir, projectName, deployType string) bool {
	var command string
	if deployType == "1" {
		command = "npx wrangler worker list"
	} else {
		command = "npx wrangler pages project list"
	}

	output, err := runCommand(installDir, command)
	if err != nil {
		warnMessage(fmt.Sprintf("Could not list Cloudflare projects to check availability: %v. Assuming project name might exist.", err))
		return true
	}

	return strings.Contains(output, projectName)
}


func extractURL(output string) (string, error) {
	re, err := regexp.Compile(`https://[-a-zA-Z0-9]+\.workers\.dev`)
	if err != nil {
		return "", fmt.Errorf("failed to compile regex: %v", err)
	}
	match := re.FindString(output)
	if match == "" {
		return "", fmt.Errorf("no Cloudflare worker URL found in output")
	}
	return match, nil
}


func extractOAuthURL(output string) (string, error) {
	re, err := regexp.Compile(`https://dash\.cloudflare\.com/oauth2/auth\?[^ \n]+`)
	if err != nil {
		return "", fmt.Errorf("failed to compile regex: %v", err)
	}
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 1 {
		return matches[0], nil
	}
	return "", fmt.Errorf("no OAuth URL found")
}

func openURL(url string) error {
	var cmd *exec.Cmd
	if _, err := os.Stat("/data/data/com.termux/files/usr/bin/termux-open-url"); err == nil {
		cmd = exec.Command("termux-open-url", url)
	} else {
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open URL: %v", err)
	}
	return nil
}


func buildWranglerConfig(filePath string) error {
	compatibilityDate := "2024-01-01"

	config := map[string]any{
		"name":                projectName,
		"compatibility_date":  compatibilityDate,
		"compatibility_flags": []string{"nodejs_compat"},
		"kv_namespaces": []map[string]string{
			{
				"binding": "kv",
				"id":      kvID,
			},
		},
		"vars": map[string]string{
			"UUID":     UUID,
			"TR_PASS":  TR_PASS,
			"PROXY_IP": PROXY_IP,
			"FALLBACK": FALLBACK,
			"SUB_PATH": SUB_PATH,
		},
	}
	if deployType == "1" {
		config["main"] = "./src/worker.js"
		config["workers_dev"] = true
	} else {
		config["pages_build_output_dir"] = "./src/"
	}
	if customDomain != "" {
		config["routes"] = []map[string]any{
			{
				"route": customDomain,
				"zone_id": "",
			},
		}
		warnMessage("Custom domain support requires manual input of Zone ID in wrangler.json after deployment.")
	}
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling config to JSON: %v", err)
	}
	if err = os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("error writing JSON to file: %v", err)
	}
	return nil
}


func extractKvID(output string) (string, error) {
	re, err := regexp.Compile(`"id":\s*"([a-f0-9]{32})"`)
	if err != nil {
		return "", fmt.Errorf("failed to compile regex: %v", err)
	}
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1], nil
	}
	return "", fmt.Errorf("no valid KV ID found in output: %s", output)
}


func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error making GET request to %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file from %s: %s (HTTP %d)", url, resp.Status, resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("error creating file %s: %v", dest, err)
	}
	defer out.Close()
	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("error writing to file %s: %v", dest, err)
	}
	return nil
}

func failMessage(message string, err error) {
	if err != nil {
		message += ": " + err.Error()
	}
	fmt.Printf("%s %s\n", errorPrefix, message)
}

func successMessage(message string) {
	fmt.Printf("%s %s\n", successPrefix, message)
}

func warnMessage(message string) {
	fmt.Printf("%s %s\n", warnPrefix, message)
}