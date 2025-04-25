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
        if err := os.Remove(wranglerConfigPath); err != nil {
            failMessage("Error deleting old worker config.", err)
            return
        }
    }

    if err := os.RemoveAll(srsPath); err != nil {
        failMessage("Error deleting old worker.js file.", err)
        return
    }

    if err := os.MkdirAll(installDir, 0750); err != nil {
        failMessage("Error creating install directory", err)
        return
    }

    fmt.Printf("\n%s Installing %sBPB Terminal Wizard%s...\n", titlePrefix, bold+blue, reset)

    if err := checkNode(); err != nil {
        failMessage("Node.js is not installed or outdated. Please install Node.js 18 or higher.", err)
        return
    }

    fmt.Printf("%s Installing Wrangler...\n", infoPrefix)
    if _, err := runCommand(installDir, "npm cache clean --force", 1); err != nil {
        fmt.Printf("%s Warning: Could not clean npm cache, continuing anyway...\n", warnPrefix)
    }
    if _, err := runCommand(installDir, "npm uninstall -g wrangler", 1); err != nil {
        fmt.Printf("%s Warning: Could not uninstall old Wrangler, continuing anyway...\n", warnPrefix)
    }
    output, err := runCommand(installDir, "npm install -g wrangler@4.12.0", 3)
    if err != nil {
        failMessage("Error installing Wrangler", fmt.Errorf("output: %s, error: %v", output, err))
        return
    }
    output, err = runCommand(installDir, "npx wrangler --version", 1)
    if err != nil || !strings.Contains(output, "4.12.0") {
        failMessage("Failed to verify Wrangler version 4.12.0", fmt.Errorf("output: %s, error: %v", output, err))
        return
    }

    successMessage("BPB Terminal Wizard dependencies are ready!")

    fmt.Printf("\n%s Starting Cloudflare login process...\n", titlePrefix)
    for {
        cmd := exec.Command("sh", "-c", "npx wrangler login")
        cmd.Dir = installDir
        var stdoutBuf bytes.Buffer
        cmd.Stdout = &stdoutBuf
        cmd.Stderr = os.Stderr
        cmd.Stdin = os.Stdin
        if err := cmd.Start(); err != nil {
            failMessage("Error starting Cloudflare login", err)
            continue
        }

        timeout := time.After(60 * time.Second)
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()

        var oauthURL string
        for {
            select {
            case <-timeout:
                fmt.Printf("%s Debug: Wrangler output: %s\n", infoPrefix, stdoutBuf.String())
                failMessage("Timeout waiting for OAuth URL", nil)
                return
            case <-ticker.C:
                oauthURL, err = extractOAuthURL(stdoutBuf.String())
                if err == nil {
                    fmt.Printf("%s Found OAuth URL: %s%s%s\n", infoPrefix, blue, oauthURL, reset)
                    if err := openURL(oauthURL); err != nil {
                        fmt.Printf("%s Could not open browser automatically.\nPlease open this URL manually: %s%s%s\n", warnPrefix, blue, oauthURL, reset)
                    } else {
                        fmt.Printf("%s Browser opened with URL: %s%s%s\n", infoPrefix, blue, oauthURL, reset)
                    }
                    goto FoundURL
                }
            }
        }
    FoundURL:

        if err := cmd.Wait(); err != nil {
            failMessage("Error logging into Cloudflare", err)
            continue
        }

        if _, err := runCommand(installDir, "npx wrangler telemetry disable", 1); err != nil {
            fmt.Printf("%s Warning: Could not disable telemetry, continuing anyway...\n", warnPrefix)
        }

        successMessage("Successfully logged into Cloudflare!")
        break
    }

    fmt.Printf("\n%s Configuring Worker settings...\n", titlePrefix)

    fmt.Printf("\n%s Using deployment type: %s%s%s\n", infoPrefix, bold+green, map[string]string{"1": "Workers", "2": "Pages"}[deployType], reset)
    if deployType == "2" {
        fmt.Printf("%s With %sPages%s, you cannot modify settings later from Cloudflare dashboard.\n", warnPrefix, bold+green, reset)
        fmt.Printf("%s With %sPages%s, it may take up to 5 minutes to access the panel.\n", warnPrefix, bold+green, reset)
    }

    for {
        projectName = generateRandomDomain(32)
        fmt.Printf("\n%s Generated worker name (%sSubdomain%s): %s%s%s\n", infoPrefix, bold+green, reset, cyan, projectName, reset)
        successMessage("Using generated worker name.")

        fmt.Printf("\n%s Checking domain availability...\n", infoPrefix)
        if resp := isWorkerAvailable(installDir, projectName, deployType); resp {
            continue
        }
        successMessage("Domain is available!")
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
    if err := os.Mkdir(srsPath, 0750); err != nil {
        failMessage("Could not create src directory", err)
        return
    }

    var workerPath = filepath.Join(srsPath, "worker.js")
    if deployType == "2" {
        workerPath = filepath.Join(srsPath, "_worker.js")
    }
    for attempt := 1; attempt <= 3; attempt++ {
        if err := downloadFile(workerURL, workerPath, 3); err == nil {
            successMessage("Worker downloaded successfully!")
            break
        }
        if attempt < 3 {
            fmt.Printf("%s Retrying download in 5 seconds...\n", infoPrefix)
            time.Sleep(5 * time.Second)
        } else {
            failMessage("Failed to download worker.js after multiple attempts.", err)
            return
        }
    }

    fmt.Printf("\n%s This program creates a new KV namespace each time it runs.\n   Check your Cloudflare account and delete unused KV namespaces to avoid limits.\n", warnPrefix)
    fmt.Printf("\n%s Creating KV namespace...\n", titlePrefix)
    for attempt := 1; attempt <= 3; attempt++ {
        kvName := fmt.Sprintf("panel_kv_%s", generateRandomString("abcdefghijklmnopqrstuvwxyz0123456789", 8, false))
        output, err := runCommand(installDir, fmt.Sprintf("npx wrangler kv namespace create %s", kvName), 3)
        if err != nil {
            message := fmt.Sprintf("Error creating KV on attempt %d! Output: %s. Check logs at ~/.config/.wrangler/logs/ for details. Ensure Wrangler is version 4.12.0.", attempt, output)
            if strings.Contains(output, "fetch failed") && attempt < 3 {
                fmt.Printf("%s Retrying after 5 seconds...\n", infoPrefix)
                time.Sleep(5 * time.Second)
                continue
            }
            failMessage(message, err)
            continue
        }

        id, err := extractKvID(output)
        if err != nil {
            message := fmt.Sprintf("Error getting KV ID! Output: %s. Check logs at ~/.config/.wrangler/logs/ for details. Ensure Wrangler is version 4.12.0.", output)
            failMessage(message, err)
            continue
        }

        kvID = id
        break
    }
    if kvID == "" {
        failMessage("Failed to create KV namespace after multiple attempts.", nil)
        return
    }
    successMessage("KV namespace created successfully!")

    fmt.Printf("\n%s Building panel configuration...\n", titlePrefix)
    if err := buildWranglerConfig(wranglerConfigPath); err != nil {
        failMessage("Error building Wrangler configuration", err)
        return
    }
    successMessage("Panel configuration built successfully!")

    var panelURL string
    for attempt := 1; attempt <= 3; attempt++ {
        fmt.Printf("\n%s Deploying %sBPB Panel%s (Attempt %d)...\n", titlePrefix, bold+blue, reset, attempt)
        if deployType == "1" {
            output, err := runCommand(installDir, "npx wrangler deploy ./src/worker.js", 3)
            if err != nil {
                failMessage(fmt.Sprintf("Error deploying Panel! Output: %s", output), err)
                if attempt < 3 {
                    fmt.Printf("%s Retrying deployment in 5 seconds...\n", infoPrefix)
                    time.Sleep(5 * time.Second)
                }
                continue
            }

            successMessage("Panel deployed successfully!")
            url, err := extractURL(output)
            if err != nil {
                failMessage("Error getting URL", err)
                return
            }
            panelURL = url + "/panel"
            break
        }

        if _, err := runCommand(installDir, fmt.Sprintf("npx wrangler pages project create %s --production-branch production", projectName), 3); err != nil {
            failMessage("Error creating Pages project", err)
            if attempt < 3 {
                fmt.Printf("%s Retrying in 5 seconds...\n", infoPrefix)
                time.Sleep(5 * time.Second)
            }
            continue
        }

        _, err := runCommand(installDir, "npx wrangler pages deploy --commit-dirty true --branch production", 3)
        if err != nil {
            failMessage("Error deploying Panel", err)
            if attempt < 3 {
                fmt.Printf("%s Retrying deployment in 5 seconds...\n", infoPrefix)
                time.Sleep(5 * time.Second)
            }
            continue
        }

        successMessage("Panel deployed successfully!")
        panelURL = "https://" + projectName + ".pages.dev/panel"
        break
    }

    if panelURL == "" {
        failMessage("Failed to deploy panel after multiple attempts.", nil)
        return
    }

    fmt.Printf("\n%s Panel installed successfully!\n%s Access it at: %s%s%s\n%s Copy this URL and open it in your browser to access the BPB Panel.\n", successPrefix, infoPrefix, blue, panelURL, reset, infoPrefix)
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

func runCommand(cmdDir string, command string, retries int) (string, error) {
    for attempt := 1; attempt <= retries; attempt++ {
        cmd := exec.Command("sh", "-c", command)
        cmd.Dir = cmdDir
        var stdoutBuf, stderrBuf bytes.Buffer
        cmd.Stdout = &stdoutBuf
        cmd.Stderr = &stderrBuf
        err := cmd.Run()
        output := stdoutBuf.String() + stderrBuf.String()
        if err == nil {
            return output, nil
        }
        if attempt < retries {
            fmt.Printf("%s Retrying command after error: %v\n", warnPrefix, err)
            time.Sleep(5 * time.Second)
        } else {
            return output, fmt.Errorf("command failed after %d attempts: %v", retries, err)
        }
    }
    return "", fmt.Errorf("unexpected error in runCommand")
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
            if !isDomain && i == 0 && char >= '0' && char <= '9' {
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
        command = fmt.Sprintf("npx wrangler deployments list --name %s", projectName)
    } else {
        command = fmt.Sprintf("npx wrangler pages deployment list --project-name %s", projectName)
    }
    _, err := runCommand(installDir, command, 1)
    return err == nil
}

func extractURL(output string) (string, error) {
    re, err := regexp.Compile(`https?://[^\s]+`)
    if err != nil {
        return "", err
    }
    matches := re.FindAllString(output, -1)
    if len(matches) == 0 {
        return "", fmt.Errorf("no matches found")
    }
    return matches[len(matches)-1], nil
}

func extractOAuthURL(output string) (string, error) {
    re, err := regexp.Compile(`https://dash\.cloudflare\.com/oauth2/auth\?[^ \n]+`)
    if err != nil {
        return "", err
    }
    matches := re.FindAllString(output, -1)
    if len(matches) == 0 {
        return "", fmt.Errorf("no OAuth URL found")
    }
    return matches[0], nil
}

func openURL(url string) error {
    var cmd *exec.Cmd
    if _, err := os.Stat("/data/data/com.termux"); err == nil {
        cmd = exec.Command("termux-open-url", url)
        cmd.Env = append(os.Environ(), "TERMUX_API_VERSION=0.50")
    } else {
        cmd = exec.Command("xdg-open", url)
    }
    return cmd.Run()
}

func buildWranglerConfig(filePath string) error {
    config := map[string]any{
        "name":                projectName,
        "compatibility_date":  time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
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
            },
        }
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
    re, err := regexp.Compile(`"id":\s*"([^"]+)"`)
    if err != nil {
        return "", fmt.Errorf("failed to compile regex: %v", err)
    }
    matches := re.FindStringSubmatch(output)
    if len(matches) >= 2 {
        return matches[1], nil
    }
    return "", fmt.Errorf("no valid ID found in output")
}

func downloadFile(url, dest string, retries int) error {
    for attempt := 1; attempt <= retries; attempt++ {
        resp, err := http.Get(url)
        if err != nil {
            if attempt < retries {
                fmt.Printf("%s Retrying download after error: %v\n", warnPrefix, err)
                time.Sleep(5 * time.Second)
                continue
            }
            return fmt.Errorf("error making GET request after %d attempts: %v", retries, err)
        }
        defer resp.Body.Close()
        if resp.StatusCode != http.StatusOK {
            if attempt < retries {
                fmt.Printf("%s Retrying download after HTTP %d\n", warnPrefix, resp.StatusCode)
                time.Sleep(5 * time.Second)
                continue
            }
            return fmt.Errorf("failed to download file after %d attempts: %s (HTTP %d)", retries, url, resp.StatusCode)
        }
        out, err := os.Create(dest)
        if err != nil {
            return fmt.Errorf("error creating file: %v", err)
        }
        defer out.Close()
        if _, err = io.Copy(out, resp.Body); err != nil {
            if attempt < retries {
                fmt.Printf("%s Retrying download after write error: %v\n", warnPrefix, err)
                time.Sleep(5 * time.Second)
                continue
            }
            return fmt.Errorf("error writing to file after %d attempts: %v", retries, err)
        }
        return nil
    }
    return fmt.Errorf("unexpected error in downloadFile")
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