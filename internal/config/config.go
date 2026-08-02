package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

const (
	AppName              = "jdeen"
	DefaultProfile       = "prod"
	DefaultProductionURL = "https://api.jdeen.com/v1"
	DefaultDevURL        = "https://api.jdeen.test:4021/v1"
)

type File struct {
	ActiveProfile string             `toml:"active_profile"`
	Profiles      map[string]Profile `toml:"profiles"`
}

type Profile struct {
	APIURL             string `toml:"api_url"`
	CACert             string `toml:"ca_cert,omitempty"`
	InsecureSkipVerify bool   `toml:"insecure_skip_verify,omitempty"`
}

type Runtime struct {
	ProfileName        string
	APIURL             string
	CACert             string
	InsecureSkipVerify bool
	ConfigPath         string
}

type Overrides struct {
	ProfileName        string
	APIURL             string
	CACert             string
	InsecureSkipVerify *bool
}

var pathOverride string

func SetPathForTest(path string) func() {
	previous := pathOverride
	pathOverride = path
	return func() { pathOverride = previous }
}

func Path() (string, error) {
	if pathOverride != "" {
		return pathOverride, nil
	}
	return xdg.ConfigFile(filepath.Join(AppName, "config.toml"))
}

func DefaultFile() File {
	return File{
		ActiveProfile: DefaultProfile,
		Profiles: map[string]Profile{
			"prod": {APIURL: DefaultProductionURL},
			"dev":  {APIURL: DefaultDevURL},
		},
	}
}

func Load(overrides Overrides) (Runtime, File, error) {
	path, err := Path()
	if err != nil {
		return Runtime{}, File{}, err
	}
	file := DefaultFile()
	if contents, readErr := os.ReadFile(path); readErr == nil {
		if err := toml.Unmarshal(contents, &file); err != nil {
			return Runtime{}, File{}, fmt.Errorf("read config: %w", err)
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return Runtime{}, File{}, fmt.Errorf("read config: %w", readErr)
	}
	runtime, err := Resolve(file, overrides, path)
	return runtime, file, err
}

func Save(file File) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	contents, err := toml.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Chmod(path, 0o600)
}

func Resolve(file File, overrides Overrides, path string) (Runtime, error) {
	if file.ActiveProfile == "" {
		file.ActiveProfile = DefaultProfile
	}
	if file.Profiles == nil {
		file.Profiles = DefaultFile().Profiles
	}
	profileName := first(overrides.ProfileName, os.Getenv("JDEEN_PROFILE"), file.ActiveProfile, DefaultProfile)
	profile, ok := file.Profiles[profileName]
	if !ok {
		return Runtime{}, fmt.Errorf("profile %q is not configured", profileName)
	}
	apiURL := first(overrides.APIURL, os.Getenv("JDEEN_API_URL"), profile.APIURL)
	caCert := first(overrides.CACert, os.Getenv("JDEEN_CA_CERT"), profile.CACert)
	insecure := profile.InsecureSkipVerify
	if raw := strings.TrimSpace(os.Getenv("JDEEN_INSECURE_SKIP_VERIFY")); raw != "" {
		insecure = raw == "1" || strings.EqualFold(raw, "true")
	}
	if overrides.InsecureSkipVerify != nil {
		insecure = *overrides.InsecureSkipVerify
	}
	normalized, err := NormalizeAPIURL(apiURL)
	if err != nil {
		return Runtime{}, err
	}
	if insecure && productionURL(normalized) {
		return Runtime{}, errors.New("insecure TLS verification is not allowed for api.jdeen.com")
	}
	return Runtime{ProfileName: profileName, APIURL: normalized, CACert: caCert, InsecureSkipVerify: insecure, ConfigPath: path}, nil
}

func NormalizeAPIURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse API URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("API URL must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("API URL must include a host")
	}
	parsed.RawQuery, parsed.Fragment = "", ""
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		path = "/v1"
	} else if !strings.HasSuffix(path, "/v1") {
		path += "/v1"
	}
	parsed.Path = path
	return parsed.String(), nil
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func productionURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && strings.EqualFold(parsed.Hostname(), "api.jdeen.com")
}
