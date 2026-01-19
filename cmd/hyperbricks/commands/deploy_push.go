package commands

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/parser"
	"github.com/mitchellh/mapstructure"
)

type deployPushConfig struct {
	HMACSecret string                 `mapstructure:"hmac_secret"`
	Remote     deployPushRemoteConfig `mapstructure:"remote"`
	Client     deployClientConfig     `mapstructure:"client"`
}

type deployPushRemoteConfig struct {
	Root    string `mapstructure:"root"`
	APIPort int    `mapstructure:"api_port"`
}

type deployClientConfig struct {
	Target  string                        `mapstructure:"target"`
	Targets map[string]deployClientTarget `mapstructure:"targets"`
}

type deployClientTarget struct {
	Host string `mapstructure:"host"`
	User string `mapstructure:"user"`
	Port int    `mapstructure:"port"`
	Root string `mapstructure:"root"`
	API  string `mapstructure:"api"`
}

type deployActivatePayload struct {
	BuildID string `json:"build_id"`
}

func runBuildPush(result buildResult) error {
	if !result.Built {
		fmt.Println("No new build created; skipping push.")
		return nil
	}

	configPath := deployConfigPath()
	cfg, err := loadDeployPushConfig(configPath)
	if err != nil {
		return err
	}
	if len(cfg.Client.Targets) == 0 {
		return fmt.Errorf("deploy.client.targets is not configured in %s", configPath)
	}

	reader := bufio.NewReader(os.Stdin)
	targetName, target, err := selectDeployTarget(cfg, buildPushTarget, reader)
	if err != nil {
		return err
	}
	if targetName == "" {
		fmt.Println("Push canceled.")
		return nil
	}

	explicitTarget := strings.TrimSpace(buildPushTarget) != ""
	if !explicitTarget {
		confirm, err := promptYesNoDefault(reader, fmt.Sprintf("Push build %s to %s? (Y/n): ", result.BuildID, targetName), true)
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Push canceled.")
			return nil
		}
	}

	resolved, err := normalizeDeployTarget(cfg, target)
	if err != nil {
		return err
	}

	archivePath := result.ArchivePath
	if !filepath.IsAbs(archivePath) {
		archivePath, err = filepath.Abs(archivePath)
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(archivePath); err != nil {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	fmt.Printf("Preparing deploy directories on %s...\n", targetName)
	if err := ensureRemoteDeployDirs(resolved, result.Module); err != nil {
		return err
	}

	fmt.Printf("Uploading %s to %s...\n", filepath.Base(archivePath), targetName)
	if err := scpArchive(resolved, archivePath, result.Module); err != nil {
		return err
	}

	secret := resolveDeploySecret(cfg)
	if secret == "" {
		return errors.New("deploy.hmac_secret or HB_DEPLOY_SECRET is required for activation")
	}

	fmt.Printf("Activating build %s on %s...\n", result.BuildID, targetName)
	if err := activateRemoteBuild(resolved, result.Module, result.BuildID, secret); err != nil {
		return err
	}

	fmt.Printf("Push complete for %s (%s).\n", result.Module, result.BuildID)
	return nil
}

func PushBuildToTarget(module string, buildID string, archivePath string, targetName string) (string, error) {
	module = strings.TrimSpace(module)
	buildID = strings.TrimSpace(buildID)
	archivePath = strings.TrimSpace(archivePath)
	if module == "" {
		return "", errors.New("module is required for push")
	}
	if buildID == "" {
		return "", errors.New("build_id is required for push")
	}
	if archivePath == "" {
		return "", errors.New("archive path is required for push")
	}

	configPath := deployConfigPath()
	cfg, err := loadDeployPushConfig(configPath)
	if err != nil {
		return "", err
	}
	if len(cfg.Client.Targets) == 0 {
		return "", fmt.Errorf("deploy.client.targets is not configured in %s", configPath)
	}

	resolvedName, target, err := resolveDeployTarget(cfg, targetName)
	if err != nil {
		return "", err
	}

	resolved, err := normalizeDeployTarget(cfg, target)
	if err != nil {
		return "", err
	}

	if !filepath.IsAbs(archivePath) {
		archivePath, err = filepath.Abs(archivePath)
		if err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(archivePath); err != nil {
		return "", fmt.Errorf("archive not found: %s", archivePath)
	}

	if err := ensureRemoteDeployDirs(resolved, module); err != nil {
		return "", err
	}

	if err := scpArchive(resolved, archivePath, module); err != nil {
		return "", err
	}

	secret := resolveDeploySecret(cfg)
	if secret == "" {
		return "", errors.New("deploy.hmac_secret or HB_DEPLOY_SECRET is required for activation")
	}

	if err := activateRemoteBuild(resolved, module, buildID, secret); err != nil {
		return "", err
	}

	return resolvedName, nil
}

func deployConfigPath() string {
	if envPath := strings.TrimSpace(os.Getenv("HB_DEPLOY_CONFIG")); envPath != "" {
		return envPath
	}
	return "deploy.hyperbricks"
}

func loadDeployPushConfig(path string) (deployPushConfig, error) {
	cfg := deployPushConfig{
		Remote: deployPushRemoteConfig{
			APIPort: 9090,
		},
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read deploy config %s: %w", path, err)
	}

	parsed := parser.ParseHyperScript(string(content))
	deployRaw, ok := parsed["deploy"].(map[string]interface{})
	if !ok {
		if hyper, ok := parsed["hyperbricks"].(map[string]interface{}); ok {
			deployRaw, _ = hyper["deploy"].(map[string]interface{})
		}
	}
	if deployRaw == nil {
		return cfg, fmt.Errorf("missing deploy block in %s", path)
	}
	if _, ok := deployRaw["remote"].(map[string]interface{}); !ok {
		return cfg, fmt.Errorf("missing deploy.remote block in %s", path)
	}
	if _, ok := deployRaw["client"].(map[string]interface{}); !ok {
		return cfg, fmt.Errorf("missing deploy.client block in %s", path)
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &cfg,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	})
	if err != nil {
		return cfg, err
	}
	if err := decoder.Decode(deployRaw); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func selectDeployTarget(cfg deployPushConfig, explicit string, reader *bufio.Reader) (string, deployClientTarget, error) {
	targetName := strings.TrimSpace(explicit)
	if targetName == "" {
		targetName = strings.TrimSpace(cfg.Client.Target)
	}

	targets := cfg.Client.Targets
	if targetName == "" && len(targets) == 1 {
		for name := range targets {
			targetName = name
		}
	}

	if targetName == "" {
		names := make([]string, 0, len(targets))
		for name := range targets {
			names = append(names, name)
		}
		sort.Strings(names)
		input, err := promptInput(reader, fmt.Sprintf("Select target (%s): ", strings.Join(names, ", ")))
		if err != nil {
			return "", deployClientTarget{}, err
		}
		targetName = strings.TrimSpace(input)
		if targetName == "" {
			return "", deployClientTarget{}, nil
		}
	}

	target, ok := targets[targetName]
	if !ok {
		return "", deployClientTarget{}, fmt.Errorf("deploy target not found: %s", targetName)
	}
	return targetName, target, nil
}

func resolveDeployTarget(cfg deployPushConfig, explicit string) (string, deployClientTarget, error) {
	targetName := strings.TrimSpace(explicit)
	if targetName == "" {
		targetName = strings.TrimSpace(cfg.Client.Target)
	}

	targets := cfg.Client.Targets
	if targetName == "" && len(targets) == 1 {
		for name := range targets {
			targetName = name
		}
	}
	if targetName == "" {
		return "", deployClientTarget{}, errors.New("deploy target is required")
	}

	target, ok := targets[targetName]
	if !ok {
		return "", deployClientTarget{}, fmt.Errorf("deploy target not found: %s", targetName)
	}
	return targetName, target, nil
}

func normalizeDeployTarget(cfg deployPushConfig, target deployClientTarget) (deployClientTarget, error) {
	target.Host = strings.TrimSpace(target.Host)
	if target.Host == "" {
		return target, errors.New("deploy target host is required")
	}
	if target.Port == 0 {
		target.Port = 22
	}
	target.User = strings.TrimSpace(target.User)
	if target.User == "" {
		if current := strings.TrimSpace(os.Getenv("USER")); current != "" {
			target.User = current
		}
	}
	target.Root = strings.TrimSpace(target.Root)
	if target.Root == "" {
		target.Root = strings.TrimSpace(cfg.Remote.Root)
	}
	if target.Root == "" {
		return target, errors.New("deploy target root is required")
	}
	target.API = strings.TrimSpace(target.API)
	if target.API == "" {
		port := cfg.Remote.APIPort
		if port == 0 {
			port = 9090
		}
		target.API = fmt.Sprintf("http://%s:%d", target.Host, port)
	}
	return target, nil
}

func resolveDeploySecret(cfg deployPushConfig) string {
	secret := strings.TrimSpace(cfg.HMACSecret)
	if secret == "" || strings.Contains(secret, "{{") {
		secret = strings.TrimSpace(os.Getenv("HB_DEPLOY_SECRET"))
	}
	return secret
}

func ensureRemoteDeployDirs(target deployClientTarget, module string) error {
	base := remoteJoin(target.Root, module)
	incoming := remoteJoin(base, "incoming")
	archives := remoteJoin(base, "archives")
	runtime := remoteJoin(base, "runtime")
	command := fmt.Sprintf("mkdir -p %s %s %s", shellQuote(incoming), shellQuote(archives), shellQuote(runtime))
	return runSSH(target, command)
}

func scpArchive(target deployClientTarget, localPath string, module string) error {
	remoteDir := remoteJoin(target.Root, module, "incoming")
	dest := fmt.Sprintf("%s:%s/", remoteHost(target), remoteDir)

	args := []string{}
	if target.Port > 0 {
		args = append(args, "-P", strconv.Itoa(target.Port))
	}
	args = append(args, localPath, dest)

	cmd := exec.Command("scp", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func activateRemoteBuild(target deployClientTarget, module string, buildID string, secret string) error {
	apiURL, err := buildActivateURL(target.API, module)
	if err != nil {
		return err
	}
	payload := deployActivatePayload{BuildID: buildID}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	headers, err := signDeployHeaders(http.MethodPost, apiURL.Path, body, secret)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var payload map[string]interface{}
		if err := json.Unmarshal(responseBody, &payload); err == nil {
			if message, ok := payload["error"].(string); ok && message != "" {
				return fmt.Errorf("deploy activation failed: %s", message)
			}
		}
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("deploy activation failed: %s", message)
	}
	return nil
}

func buildActivateURL(base string, module string) (*url.URL, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return nil, errors.New("deploy target api is required")
	}
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid deploy api url: %s", base)
	}
	joined := path.Join(parsed.Path, "deploy", "modules", module, "activate")
	parsed.Path = "/" + strings.TrimPrefix(joined, "/")
	parsed.RawQuery = ""
	return parsed, nil
}

func signDeployHeaders(method string, requestPath string, body []byte, secret string) (map[string]string, error) {
	nonce, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	bodyHash := sha256.Sum256(body)
	canonical := strings.Join([]string{
		method,
		requestPath,
		hex.EncodeToString(bodyHash[:]),
		ts,
		nonce,
	}, "\n")

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	signature := hex.EncodeToString(mac.Sum(nil))

	return map[string]string{
		"X-HB-Timestamp": ts,
		"X-HB-Nonce":     nonce,
		"X-HB-Signature": signature,
	}, nil
}

func randomHex(bytesLen int) (string, error) {
	data := make([]byte, bytesLen)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func remoteJoin(parts ...string) string {
	return path.Clean(path.Join(parts...))
}

func remoteHost(target deployClientTarget) string {
	if target.User == "" {
		return target.Host
	}
	return fmt.Sprintf("%s@%s", target.User, target.Host)
}

func runSSH(target deployClientTarget, command string) error {
	args := []string{}
	if target.Port > 0 {
		args = append(args, "-p", strconv.Itoa(target.Port))
	}
	args = append(args, remoteHost(target), command)

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
