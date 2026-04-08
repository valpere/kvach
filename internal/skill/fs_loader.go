package skill

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FSLoader discovers skills from the filesystem.
type FSLoader struct {
	homeDir    string
	projectDir string
	extraDirs  []string

	entries map[string]CatalogEntry
	paths   map[string]string
}

// NewFSLoader returns a filesystem skill loader.
func NewFSLoader(homeDir string) *FSLoader {
	return &FSLoader{
		homeDir: homeDir,
		entries: make(map[string]CatalogEntry),
		paths:   make(map[string]string),
	}
}

// Discover implements [Loader].
func (l *FSLoader) Discover(projectDir string, extraDirs []string) ([]CatalogEntry, error) {
	l.projectDir = projectDir
	l.extraDirs = extraDirs
	l.entries = make(map[string]CatalogEntry)
	l.paths = make(map[string]string)

	paths := SearchPaths(l.homeDir, projectDir, nil)
	for _, d := range extraDirs {
		paths = append(paths, struct {
			Dir    string
			Source string
		}{Dir: d, Source: "extra"})
	}

	for _, p := range paths {
		if err := l.discoverInDir(p.Dir, p.Source); err != nil {
			return nil, err
		}
	}

	entries := make([]CatalogEntry, 0, len(l.entries))
	for _, e := range l.entries {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

// Activate implements [Loader].
func (l *FSLoader) Activate(name string) (*Skill, error) {
	path, ok := l.paths[name]
	if !ok {
		if l.projectDir != "" {
			if _, err := l.Discover(l.projectDir, l.extraDirs); err != nil {
				return nil, err
			}
			path, ok = l.paths[name]
		}
	}
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return l.ParseFile(path)
}

// ParseFile implements [Loader].
func (l *FSLoader) ParseFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill file %s: %w", path, err)
	}

	fmRaw, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("parse skill frontmatter %s: %w", path, err)
	}

	var fm Frontmatter
	if err := yaml.Unmarshal(fmRaw, &fm); err != nil {
		return nil, fmt.Errorf("unmarshal skill frontmatter %s: %w", path, err)
	}
	if err := ValidateName(fm.Name); err != nil {
		return nil, err
	}
	if strings.TrimSpace(fm.Description) == "" {
		return nil, errors.New("skill description is required")
	}

	baseDir := filepath.Dir(path)
	if filepath.Base(baseDir) != fm.Name {
		return nil, fmt.Errorf("skill name %q must match directory %q", fm.Name, filepath.Base(baseDir))
	}

	resources, err := collectResources(baseDir)
	if err != nil {
		return nil, err
	}
	libraries, err := collectLibraries(baseDir)
	if err != nil {
		return nil, err
	}
	cfg, cfgPath, err := readSkillConfig(baseDir)
	if err != nil {
		return nil, err
	}

	return &Skill{
		Frontmatter: fm,
		Location:    path,
		BaseDir:     baseDir,
		Body:        strings.TrimSpace(string(body)),
		Resources:   resources,
		Config:      cfg,
		ConfigPath:  cfgPath,
		Libraries:   libraries,
	}, nil
}

func (l *FSLoader) discoverInDir(dir, source string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read skill dir %s: %w", dir, err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillFile := filepath.Join(dir, e.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			continue
		}

		skillObj, err := l.ParseFile(skillFile)
		if err != nil {
			continue
		}
		skillObj.Source = source

		l.entries[skillObj.Name] = CatalogEntry{
			Name:        skillObj.Name,
			Description: skillObj.Description,
			Location:    skillObj.Location,
		}
		l.paths[skillObj.Name] = skillObj.Location
	}
	return nil
}

func splitFrontmatter(data []byte) (frontmatter, body []byte, err error) {
	trimmed := bytes.TrimSpace(data)
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return nil, nil, errors.New("missing opening frontmatter delimiter")
	}
	rest := trimmed[3:]
	if len(rest) > 0 && rest[0] == '\r' {
		rest = rest[1:]
	}
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return nil, nil, errors.New("missing closing frontmatter delimiter")
	}
	frontmatter = rest[:idx]
	body = rest[idx+4:]
	if len(body) > 0 && body[0] == '\r' {
		body = body[1:]
	}
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}
	return frontmatter, body, nil
}

func collectResources(baseDir string) ([]string, error) {
	resourceDirs := []string{"scripts", "references", "assets"}
	var out []string
	for _, rd := range resourceDirs {
		path := filepath.Join(baseDir, rd)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		}
		err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(baseDir, p)
			if err == nil {
				out = append(out, rel)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan resources in %s: %w", path, err)
		}
	}
	sort.Strings(out)
	return out, nil
}

func collectLibraries(baseDir string) ([]string, error) {
	libDir := filepath.Join(baseDir, "lib")
	if _, err := os.Stat(libDir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var out []string
	err := filepath.WalkDir(libDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(baseDir, p)
		if err == nil {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan libraries in %s: %w", libDir, err)
	}
	sort.Strings(out)
	return out, nil
}

func readSkillConfig(baseDir string) (map[string]any, string, error) {
	yamlPath := filepath.Join(baseDir, "config.yaml")
	if data, err := os.ReadFile(yamlPath); err == nil {
		var cfg map[string]any
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, "", fmt.Errorf("parse %s: %w", yamlPath, err)
		}
		return cfg, yamlPath, nil
	}

	jsonPath := filepath.Join(baseDir, "config.json")
	if data, err := os.ReadFile(jsonPath); err == nil {
		var cfg map[string]any
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, "", fmt.Errorf("parse %s: %w", jsonPath, err)
		}
		return cfg, jsonPath, nil
	}

	return nil, "", nil
}
