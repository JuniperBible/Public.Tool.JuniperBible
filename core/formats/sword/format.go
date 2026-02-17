// Package sword provides canonical SWORD Bible format support.
// SWORD detects module directories with mods.d/*.conf files and modules/* data directories.
//
// IR Support:
// - extract-ir: Extracts IR from SWORD module (L2 - requires libsword for full text)
// - emit-native: Converts IR back to SWORD module format (L2)
package sword

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/plugins/ipc"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/format"
	"github.com/FocuswithJustin/JuniperBible/plugins/sdk/ir"
)

// Config defines the SWORD format plugin.
var Config = &format.Config{
	PluginID:   "format.sword",
	Name:       "SWORD",
	Extensions: []string{},
	Detect:     detectSword,
	Parse:      parseSword,
	Emit:       emitSword,
	Enumerate:  enumerateSword,
}

// SwordModule represents parsed SWORD module metadata.
type SwordModule struct {
	Name        string
	Description string
	Version     string
	DataPath    string
	ConfPath    string
	ModDrv      string
	Lang        string
	Encoding    string
}

func detectSword(path string) (*ipc.DetectResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &ipc.DetectResult{Detected: false, Reason: fmt.Sprintf("cannot stat: %v", err)}, nil
	}

	if !info.IsDir() {
		return &ipc.DetectResult{Detected: false, Reason: "path is not a directory"}, nil
	}

	modsD := filepath.Join(path, "mods.d")
	if _, err := os.Stat(modsD); errors.Is(err, os.ErrNotExist) {
		return &ipc.DetectResult{Detected: false, Reason: "no mods.d directory found"}, nil
	}

	confFiles, err := filepath.Glob(filepath.Join(modsD, "*.conf"))
	if err != nil || len(confFiles) == 0 {
		return &ipc.DetectResult{Detected: false, Reason: "no .conf files in mods.d/"}, nil
	}

	modulesDir := filepath.Join(path, "modules")
	if _, err := os.Stat(modulesDir); errors.Is(err, os.ErrNotExist) {
		return &ipc.DetectResult{Detected: false, Reason: "no modules directory found"}, nil
	}

	return &ipc.DetectResult{
		Detected: true,
		Format:   "SWORD",
		Reason:   fmt.Sprintf("SWORD module detected: %d .conf file(s)", len(confFiles)),
	}, nil
}

func enumerateSword(path string) (*ipc.EnumerateResult, error) {
	modules, _ := parseSwordModules(path)
	moduleMap := make(map[string]*SwordModule)
	for _, m := range modules {
		moduleMap[m.ConfPath] = m
	}

	var entries []ipc.EnumerateEntry

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(path, p)
		if rel == "." {
			return nil
		}

		entry := ipc.EnumerateEntry{
			Path:      rel,
			SizeBytes: info.Size(),
			IsDir:     info.IsDir(),
		}

		if strings.HasSuffix(rel, ".conf") && strings.HasPrefix(rel, "mods.d/") {
			if m, ok := moduleMap[p]; ok {
				entry.Metadata = map[string]string{
					"module_name":    m.Name,
					"description":    m.Description,
					"module_version": m.Version,
				}
			}
		}

		entries = append(entries, entry)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to enumerate: %w", err)
	}

	return &ipc.EnumerateResult{Entries: entries}, nil
}

func parseSword(path string) (*ir.Corpus, error) {
	modules, err := parseSwordModules(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse modules: %w", err)
	}

	if len(modules) == 0 {
		return nil, fmt.Errorf("no SWORD modules found")
	}

	module := modules[0]
	corpus := ir.NewCorpus(module.Name, "BIBLE", "")
	corpus.SourceFormat = "SWORD"
	corpus.Language = module.Lang
	corpus.Title = module.Description
	corpus.LossClass = "L2"
	corpus.Attributes = map[string]string{
		"_sword_module_name": module.Name,
		"_sword_data_path":   module.DataPath,
		"_sword_mod_drv":     module.ModDrv,
		"_sword_version":     module.Version,
	}

	if module.Encoding != "" {
		corpus.Attributes["_sword_encoding"] = module.Encoding
	}

	// Read conf file for L0 reconstruction
	if confData, err := os.ReadFile(module.ConfPath); err == nil {
		corpus.Attributes["_sword_conf"] = string(confData)
		h := sha256.Sum256(confData)
		corpus.SourceHash = hex.EncodeToString(h[:])
	}

	// Placeholder document (full text requires libsword)
	corpus.Documents = []*ir.Document{
		ir.NewDocument(module.Name, module.Description, 1),
	}
	corpus.Documents[0].Attributes = map[string]string{
		"note": "Full text extraction requires tool-libsword plugin",
	}

	return corpus, nil
}

func parseSwordModules(path string) ([]*SwordModule, error) {
	modsD := filepath.Join(path, "mods.d")
	confFiles, err := filepath.Glob(filepath.Join(modsD, "*.conf"))
	if err != nil {
		return nil, err
	}

	var modules []*SwordModule
	for _, confPath := range confFiles {
		m, err := parseConfFile(confPath)
		if err != nil {
			continue
		}
		modules = append(modules, m)
	}

	return modules, nil
}

func parseConfFile(path string) (*SwordModule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	module := &SwordModule{ConfPath: path}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			module.Name = line[1 : len(line)-1]
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Description":
			module.Description = value
		case "Version":
			module.Version = value
		case "DataPath":
			module.DataPath = value
		case "ModDrv":
			module.ModDrv = value
		case "Lang":
			module.Lang = value
		case "Encoding":
			module.Encoding = value
		}
	}

	if module.Name == "" {
		return nil, fmt.Errorf("no module name found")
	}

	return module, nil
}

func emitSword(corpus *ir.Corpus, outputDir string) (string, error) {
	moduleDir := filepath.Join(outputDir, corpus.ID)
	modsD := filepath.Join(moduleDir, "mods.d")
	modulesDir := filepath.Join(moduleDir, "modules")

	if err := os.MkdirAll(modsD, 0700); err != nil {
		return "", fmt.Errorf("failed to create mods.d: %w", err)
	}
	if err := os.MkdirAll(modulesDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create modules: %w", err)
	}

	// Check for original conf file
	if confContent, ok := corpus.Attributes["_sword_conf"]; ok && confContent != "" {
		confPath := filepath.Join(modsD, strings.ToLower(corpus.ID)+".conf")
		if err := os.WriteFile(confPath, []byte(confContent), 0600); err != nil {
			return "", fmt.Errorf("failed to write conf: %w", err)
		}
	} else {
		// Generate minimal conf file
		var confBuf strings.Builder
		confBuf.WriteString(fmt.Sprintf("[%s]\n", corpus.ID))
		if dataPath, ok := corpus.Attributes["_sword_data_path"]; ok {
			confBuf.WriteString(fmt.Sprintf("DataPath=%s\n", dataPath))
		} else {
			confBuf.WriteString(fmt.Sprintf("DataPath=./modules/texts/ztext/%s/\n", strings.ToLower(corpus.ID)))
		}
		if modDrv, ok := corpus.Attributes["_sword_mod_drv"]; ok {
			confBuf.WriteString(fmt.Sprintf("ModDrv=%s\n", modDrv))
		} else {
			confBuf.WriteString("ModDrv=zText\n")
		}
		if corpus.Title != "" {
			confBuf.WriteString(fmt.Sprintf("Description=%s\n", corpus.Title))
		}
		if corpus.Language != "" {
			confBuf.WriteString(fmt.Sprintf("Lang=%s\n", corpus.Language))
		}
		if version, ok := corpus.Attributes["_sword_version"]; ok {
			confBuf.WriteString(fmt.Sprintf("Version=%s\n", version))
		}
		if encoding, ok := corpus.Attributes["_sword_encoding"]; ok {
			confBuf.WriteString(fmt.Sprintf("Encoding=%s\n", encoding))
		}

		confPath := filepath.Join(modsD, strings.ToLower(corpus.ID)+".conf")
		if err := os.WriteFile(confPath, []byte(confBuf.String()), 0600); err != nil {
			return "", fmt.Errorf("failed to write conf: %w", err)
		}
	}

	return moduleDir, nil
}
