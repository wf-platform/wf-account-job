package envloader

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/conf"
	"gopkg.in/yaml.v3"
)

var supportedEnvironments = map[string]struct{}{
	"dev":  {},
	"test": {},
	"pre":  {},
	"prod": {},
}

// Load reads .env and optional service-specific files before config loading.
// Variables already supplied by the process environment take precedence.
func Load(services ...string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}

	protected := make(map[string]struct{})
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			protected[key] = struct{}{}
		}
	}

	values, err := readFile(filepath.Join(root, ".env"))
	if err != nil {
		return err
	}
	apply(values, protected)

	for _, service := range services {
		service = strings.TrimSpace(service)
		if service == "" {
			continue
		}
		serviceValues, err := readFile(filepath.Join(root, ".env."+service))
		if err != nil {
			return err
		}
		apply(serviceValues, protected)
	}

	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	if env == "" {
		env = "dev"
		setIfAllowed("APP_ENV", env, protected)
	}
	if _, ok := supportedEnvironments[env]; !ok {
		return fmt.Errorf("unsupported APP_ENV %q, expected one of dev, test, pre, prod", env)
	}
	apply(defaultValues(), protected)

	return nil
}

// ResolveConfigFile returns an environment-specific config path when no
// explicit path was supplied.
func ResolveConfigFile(explicit, service string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}

	root, err := projectRoot()
	if err != nil {
		root = "."
	}
	env := currentEnv()
	configName := configName(service)

	candidates := []string{
		filepath.Join(root, service, "etc", configName+"-"+env+".yaml"),
		filepath.Join(root, "etc", configName+"-"+env+".yaml"),
		filepath.Join(root, service, "etc", configName+".yaml"),
		filepath.Join(root, "etc", configName+".yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return filepath.Join(root, "etc", configName+"-"+env+".yaml")
}

// LoadConfig loads the common YAML file and overlays the active environment
// profile. An explicit file path bypasses profile merging for compatibility.
func LoadConfig(explicit, service string, target any) error {
	if strings.TrimSpace(explicit) != "" {
		return conf.Load(explicit, target, conf.UseEnv())
	}

	profileFile := ResolveConfigFile("", service)
	baseFile := resolveBaseConfigFile(service)
	if profileFile == baseFile {
		return conf.Load(profileFile, target, conf.UseEnv())
	}

	baseContent, err := os.ReadFile(baseFile)
	if err != nil {
		return err
	}
	profileContent, err := os.ReadFile(profileFile)
	if err != nil {
		return err
	}

	var baseData map[string]any
	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(baseContent))), &baseData); err != nil {
		return fmt.Errorf("parse base config %s: %w", baseFile, err)
	}

	var profileData map[string]any
	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(profileContent))), &profileData); err != nil {
		return fmt.Errorf("parse profile config %s: %w", profileFile, err)
	}

	mergeMaps(baseData, profileData)
	mergedContent, err := yaml.Marshal(baseData)
	if err != nil {
		return fmt.Errorf("merge config %s and %s: %w", baseFile, profileFile, err)
	}

	return conf.LoadFromYamlBytes(mergedContent, target)
}

func resolveBaseConfigFile(service string) string {
	root, err := projectRoot()
	if err != nil {
		root = "."
	}
	configName := configName(service)

	candidates := []string{
		filepath.Join(root, service, "etc", configName+".yaml"),
		filepath.Join(root, "etc", configName+".yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return filepath.Join(root, "etc", configName+".yaml")
}

func currentEnv() string {
	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	if env == "" {
		return "dev"
	}
	return env
}

func configName(service string) string {
	service = strings.TrimSpace(service)
	if service == "" {
		return "job"
	}
	return service
}

func mergeMaps(base, overlay map[string]any) {
	for key, value := range overlay {
		overlayMap, ok := value.(map[string]any)
		if !ok {
			base[key] = value
			continue
		}

		baseMap, ok := base[key].(map[string]any)
		if !ok {
			base[key] = overlayMap
			continue
		}

		mergeMaps(baseMap, overlayMap)
	}
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	startDir := dir

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir, nil
		}
		dir = parent
	}
}

func readFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid environment entry in %s at line %d", path, lineNumber)
		}
		parsed, err := parseValue(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("invalid environment entry in %s at line %d: %w", path, lineNumber, err)
		}
		values[strings.TrimSpace(key)] = parsed
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return values, nil
}

func parseValue(value string) (string, error) {
	if len(value) < 2 {
		return value, nil
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		return strconv.Unquote(value)
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], nil
	}
	return value, nil
}

func apply(values map[string]string, protected map[string]struct{}) {
	for key, value := range values {
		setIfAllowed(key, value, protected)
	}
}

func setIfAllowed(key, value string, protected map[string]struct{}) {
	if _, exists := protected[key]; exists || strings.TrimSpace(os.Getenv(key)) != "" {
		return
	}
	_ = os.Setenv(key, value)
}

func defaultValues() map[string]string {
	return map[string]string{
		"DATABASE_TYPE":                 "postgres",
		"DATABASE_HOST":                 "localhost",
		"DATABASE_PORT":                 "5432",
		"DATABASE_USERNAME":             "go_user",
		"DATABASE_PASSWORD":             "",
		"DATABASE_MAX_OPEN_CONN":        "100",
		"DATABASE_SSL_MODE":             "disable",
		"DATABASE_CACHE_TIME":           "5",
		"DATABASE_DEBUG":                "false",
		"REDIS_HOST":                    "127.0.0.1:6379",
		"REDIS_DB":                      "0",
		"REDIS_MODE":                    "single",
		"REDIS_USERNAME":                "",
		"REDIS_PASSWORD":                "",
		"REDIS_TLS":                     "false",
		"JOB_NAME":                      "wf-account-job.rpc",
		"JOB_LISTEN_ON":                 "0.0.0.0:9105",
		"JOB_DATABASE_DB_NAME":          "wf",
		"JOB_LOG_SERVICE_NAME":          "wfAccountJobRpcLogger",
		"JOB_LOG_MODE":                  "console",
		"JOB_LOG_PATH":                  "./data/logs/wf-account-job/rpc",
		"JOB_LOG_ENCODING":              "json",
		"JOB_LOG_LEVEL":                 "info",
		"JOB_LOG_COMPRESS":              "false",
		"JOB_LOG_KEEP_DAYS":             "7",
		"JOB_LOG_STACK_COOLDOWN_MILLIS": "100",
		"JOB_PROMETHEUS_HOST":           "0.0.0.0",
		"JOB_PROMETHEUS_PORT":           "4005",
		"JOB_PROMETHEUS_PATH":           "/metrics",
		"JOB_ASYNQ_ENABLE":              "true",
		"JOB_ASYNQ_CONCURRENCY":         "20",
		"JOB_ASYNQ_SYNC_INTERVAL":       "10",
		"JOB_ENABLE_SCHEDULED_TASK":     "false",
		"JOB_ENABLE_DP_TASK":            "true",
		"JOB_DEV_LOG_MODE":              "console",
		"JOB_DEV_LOG_LEVEL":             "debug",
		"JOB_TEST_LISTEN_ON":            "0.0.0.0:9115",
		"JOB_TEST_LOG_MODE":             "file",
		"JOB_TEST_LOG_PATH":             "./data/logs/wf-account-job/test/rpc",
		"JOB_TEST_LOG_LEVEL":            "debug",
		"JOB_TEST_DATABASE_DB_NAME":     "wf_test",
		"JOB_TEST_REDIS_DB":             "1",
		"JOB_TEST_PROMETHEUS_PORT":      "4015",
		"JOB_PRE_LISTEN_ON":             "0.0.0.0:9125",
		"JOB_PRE_LOG_MODE":              "file",
		"JOB_PRE_LOG_PATH":              "./data/logs/wf-account-job/pre/rpc",
		"JOB_PRE_LOG_KEEP_DAYS":         "14",
		"JOB_PRE_DATABASE_DB_NAME":      "wf_pre",
		"JOB_PRE_REDIS_DB":              "2",
		"JOB_PRE_PROMETHEUS_PORT":       "4025",
		"JOB_PROD_LOG_MODE":             "file",
		"JOB_PROD_LOG_PATH":             "./data/logs/wf-account-job/prod/rpc",
		"JOB_PROD_LOG_COMPRESS":         "true",
		"JOB_PROD_LOG_KEEP_DAYS":        "30",
		"JOB_PROD_DATABASE_DB_NAME":     "wf_prod",
		"JOB_PROD_REDIS_DB":             "3",
	}
}
