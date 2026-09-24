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
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type deployPushConfig = shared.DeployConfig
type deployClientTarget = shared.DeployClientTarget

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

	resolved, err := normalizeDeployTarget(target)
	if err != nil {
		return err
	}
	secret := resolveDeploySecret(resolved)
	if secret == "" {
		return errors.New("deploy.client target hmac_secret is required for push")
	}

	logging.GetLogger().Named("deploy").Infow("Uploading and activating build", "archive", filepath.Base(archivePath), "target", targetName)
	if err := uploadRemoteBuild(resolved, result.Module, result.BuildID, archivePath, secret); err != nil {
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

	resolved, err := normalizeDeployTarget(target)
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

	secret := resolveDeploySecret(resolved)
	if secret == "" {
		return "", errors.New("deploy.client target hmac_secret is required for push")
	}

	if err := uploadRemoteBuild(resolved, module, buildID, archivePath, secret); err != nil {
		return "", err
	}

	return resolvedName, nil
}

func loadDeployPushConfig(path string) (deployPushConfig, error) {
	cfg := shared.DefaultDeployConfig()
	deployRaw, err := loadDeployYAMLRoot(path)
	if err != nil {
		return cfg, err
	}
	if _, ok := deployRaw["client"].(map[string]interface{}); !ok {
		return cfg, fmt.Errorf("missing deploy.client block in %s", path)
	}
	return shared.DecodeDeployConfig(deployRaw)
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

func normalizeDeployTarget(target deployClientTarget) (deployClientTarget, error) {
	target.API = strings.TrimSpace(target.API)
	target.KeyID = strings.TrimSpace(target.KeyID)
	if target.API == "" {
		return target, errors.New("deploy target api is required")
	}
	if err := target.Credentials.Validate("deploy target"); err != nil {
		return target, err
	}
	return target, nil
}

func resolveDeploySecret(target deployClientTarget) string {
	return strings.TrimSpace(target.HMACSecret)
}

func uploadRemoteBuild(target deployClientTarget, module string, buildID string, archivePath string, secret string) error {
	apiURL, err := buildReleaseURL(target.API, module)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(archivePath)
	if err != nil {
		return err
	}

	headers, err := signDeployHeaders(http.MethodPost, apiURL.Path, body, secret, target.KeyID, buildID)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/vnd.hyperbricks.hra")
	if err := shared.ApplyBasicAuth(req, target.Credentials, "deploy target"); err != nil {
		return err
	}
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
				return fmt.Errorf("deploy upload failed: %s", message)
			}
		}
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("deploy upload failed: %s", message)
	}
	return nil
}

func buildReleaseURL(base string, module string) (*url.URL, error) {
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
	joined := path.Join(parsed.Path, "deploy", "v1", "modules", module, "releases")
	parsed.Path = "/" + strings.TrimPrefix(joined, "/")
	parsed.RawQuery = ""
	return parsed, nil
}

func signDeployHeaders(method string, requestPath string, body []byte, secret string, keyID string, buildID string) (map[string]string, error) {
	nonce, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	bodyHash := sha256.Sum256(body)
	bodyHashHex := hex.EncodeToString(bodyHash[:])
	canonicalParts := []string{
		method,
		requestPath,
		bodyHashHex,
		ts,
		nonce,
	}
	keyID = strings.TrimSpace(keyID)
	buildID = strings.TrimSpace(buildID)
	if keyID != "" || buildID != "" {
		canonicalParts = append(canonicalParts, keyID, buildID)
	}
	canonical := strings.Join(canonicalParts, "\n")

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	signature := hex.EncodeToString(mac.Sum(nil))

	headers := map[string]string{
		"X-HB-Timestamp": ts,
		"X-HB-Nonce":     nonce,
		"X-HB-Signature": signature,
		"X-HB-SHA256":    bodyHashHex,
		"X-HB-Build-ID":  buildID,
	}
	if keyID != "" {
		headers["X-HB-Key-ID"] = keyID
	}
	return headers, nil
}

func randomHex(bytesLen int) (string, error) {
	data := make([]byte, bytesLen)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
