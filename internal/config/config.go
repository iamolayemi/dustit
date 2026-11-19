// Package config loads ~/.config/macclean/config.yaml.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Category is a user-defined cleanup category: the contents of Paths are removed.
type Category struct {
	ID         string   `yaml:"id"`
	Name       string   `yaml:"name"`
	Desc       string   `yaml:"description"`
	Paths      []string `yaml:"paths"`
	Risk       string   `yaml:"risk"` // safe | review (default review)
	Trash      bool     `yaml:"trash"`
	MinAgeDays int      `yaml:"min_age_days"`
}

// Config is the user's settings. Zero values mean "use the default".
type Config struct {
	Exclude          []string   `yaml:"exclude"`
	ProjectRoots     []string   `yaml:"project_roots"`
	StaleDays        int        `yaml:"stale_days"`
	OldFileDays      int        `yaml:"old_file_days"`
	OldFileMin       string     `yaml:"old_file_min"`
	DuplicateMin     string     `yaml:"duplicate_min"`
	DuplicateFolders []string   `yaml:"duplicate_folders"`
	DownloadDays     int        `yaml:"download_days"`
	Disabled         []string   `yaml:"disabled"`
	Categories       []Category `yaml:"categories"`
}

// Current is the loaded configuration (defaults until Load succeeds).
var Current = Defaults()

// Defaults are used for anything the file leaves out.
func Defaults() Config {
	return Config{StaleDays: 30, OldFileDays: 365, OldFileMin: "100MB", DuplicateMin: "5MB", DownloadDays: 90}
}

// Path is $MACCLEAN_CONFIG, or $XDG_CONFIG_HOME/macclean/config.yaml, or ~/.config/macclean/config.yaml.
func Path() string {
	if p := os.Getenv("MACCLEAN_CONFIG"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "macclean", "config.yaml")
}

// Load reads the config file into Current. A missing file is fine.
func Load() error {
	c := Defaults()
	b, err := os.ReadFile(Path())
	if errors.Is(err, os.ErrNotExist) {
		Current = c
		return nil
	}
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return fmt.Errorf("%s: %w", Path(), err)
	}
	d := Defaults()
	if c.StaleDays <= 0 {
		c.StaleDays = d.StaleDays
	}
	if c.OldFileDays <= 0 {
		c.OldFileDays = d.OldFileDays
	}
	if c.DownloadDays <= 0 {
		c.DownloadDays = d.DownloadDays
	}
	if c.OldFileMin == "" {
		c.OldFileMin = d.OldFileMin
	}
	if c.DuplicateMin == "" {
		c.DuplicateMin = d.DuplicateMin
	}
	for i, cat := range c.Categories {
		if cat.ID == "" || len(cat.Paths) == 0 {
			return fmt.Errorf("%s: custom category %d needs an id and at least one path", Path(), i+1)
		}
		if cat.Risk != "" && cat.Risk != "safe" && cat.Risk != "review" {
			return fmt.Errorf("%s: category %q: risk must be safe or review", Path(), cat.ID)
		}
	}
	Current = c
	return nil
}

// Disabled reports whether a category id is turned off.
func (c Config) IsDisabled(id string) bool {
	for _, d := range c.Disabled {
		if strings.EqualFold(strings.TrimSpace(d), id) {
			return true
		}
	}
	return false
}

// Example is a commented starter config.
const Example = `# macclean configuration
# Docs: https://github.com/iamolayemi/macclean#configuration

# Paths macclean must never remove (or remove anything inside). Globs work.
exclude:
  # - ~/Projects/important-client/node_modules
  # - ~/Library/Caches/com.example.keepme
  # - ~/Downloads/*.dmg

# Extra folders to search for projects (in addition to ~/Projects, ~/code, ~/Documents…).
project_roots:
  # - ~/work
  # - /Volumes/Dev/repos

# Thresholds.
stale_days: 30        # stale-deps: projects untouched this long
old_file_days: 365    # old-files: files not changed this long…
old_file_min: 100MB   # …and at least this big
duplicate_min: 5MB    # duplicates: ignore smaller files
duplicate_folders:    # duplicates: only look in these folders (default: personal + project folders)
  # - ~/Pictures
  # - ~/Documents
download_days: 90     # old-downloads: downloads older than this

# Categories to turn off completely (see: macclean list).
disabled:
  # - electron-caches

# Your own categories: the contents of each path are removed.
categories:
  # - id: unity-cache
  #   name: Unity cache
  #   description: Unity's package and asset cache
  #   paths: [~/Library/Unity/cache]
  #   risk: safe          # safe = cleaned by "macclean clean"; review = only with --only/--all
  #   trash: false        # true = move to Trash instead of deleting
  #   min_age_days: 0     # only entries older than this
`

// Init writes the example config if none exists and returns its path.
func Init() (string, error) {
	p := Path()
	if _, err := os.Stat(p); err == nil {
		return p, fmt.Errorf("%s already exists", p)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return p, err
	}
	return p, os.WriteFile(p, []byte(Example), 0o644)
}
