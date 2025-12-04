package snclient

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

func init() {
	RegisterModule(&AvailableTasks, "CheckExternalScripts", "/settings/external scripts", NewExternalScriptsHandler, nil)
}

type ExternalScriptsHandler struct {
	noCopy noCopy
	snc    *Agent
}

func NewExternalScriptsHandler() Module {
	return &ExternalScriptsHandler{}
}

func (e *ExternalScriptsHandler) Init(snc *Agent, defaultScriptConfig *ConfigSection, conf *Config, runSet *AgentRunSet) error {
	e.snc = snc

	if err := e.registerScriptPath(defaultScriptConfig, conf); err != nil {
		return err
	}
	if err := e.registerScripts(conf, runSet); err != nil {
		return err
	}
	if err := e.registerWrapped(conf, runSet); err != nil {
		return err
	}

	log.Tracef("external scripts initialized")

	return nil
}

func (e *ExternalScriptsHandler) Start() error {
	return nil
}

func (e *ExternalScriptsHandler) Stop() {
}

func (e *ExternalScriptsHandler) registerScripts(conf *Config, runSet *AgentRunSet) error {
	// merge command shortcuts into separate config sections
	scripts := conf.Section("/settings/external scripts/scripts")
	for name := range scripts.data {
		cmdConf := conf.Section("/settings/external scripts/scripts/" + name)
		if !cmdConf.HasKey("command") {
			raw, _, _ := scripts.GetStringRaw(name)
			cmdConf.Set("command", strings.Join(raw, " "))
		}
	}

	// now read all scripts into available checks
	for sectionName := range conf.SectionsByPrefix("/settings/external scripts/scripts/") {
		name := path.Base(sectionName)
		if name == "default" {
			continue
		}
		cmdConf := conf.Section(sectionName)
		if command, _, ok := cmdConf.GetStringRaw("command"); ok {
			log.Tracef("registered script: %s -> %s", name, command)
			if _, ok := AvailableChecks[name]; ok {
				log.Warnf("there is a built in check with the name: %s . the external script registered on path: %s has the same base name", name, command)
			}
			runSet.cmdWraps[name] = CheckEntry{name, func() CheckHandler {
				return &CheckWrap{name: name, commandString: strings.Join(command, " "), config: cmdConf}
			}}
		} else {
			return fmt.Errorf("missing command in external script %s", name)
		}
	}

	return nil
}

func (e *ExternalScriptsHandler) registerWrapped(conf *Config, runSet *AgentRunSet) error {
	// merge wrapped command shortcuts into separate config sections
	scripts := conf.Section("/settings/external scripts/wrapped scripts")
	for name, command := range scripts.data {
		cmdConf := conf.Section("/settings/external scripts/wrapped scripts/" + name)
		if !cmdConf.HasKey("command") {
			cmdConf.Set("command", command)
		}
	}

	// now read all wrapped scripts into available checks
	for sectionName := range conf.SectionsByPrefix("/settings/external scripts/wrapped scripts/") {
		name := path.Base(sectionName)
		if name == "default" {
			continue
		}
		cmdConf := conf.Section(sectionName)
		if command, _, ok := cmdConf.GetStringRaw("command"); ok {
			log.Tracef("registered wrapped script: %s -> %s", name, command)
			if _, ok := AvailableChecks[name]; ok {
				log.Warnf("there is a built in check with the name: %s . the external wrapped script registered path: %s has the same base name", name, command)
			}
			runSet.cmdWraps[name] = CheckEntry{name, func() CheckHandler {
				return &CheckWrap{name: name, commandString: strings.Join(command, " "), wrapped: true, config: cmdConf}
			}}
		} else {
			return fmt.Errorf("missing command in wrapped external script %s", name)
		}
	}

	return nil
}

func isExecutable(mode fs.FileMode) bool {
	ownerExecutionBitmask := 0o0100
	groupExecutableBitmask := 0o0010
	othersExecutableBitmask := 0o001
	executableByOwner := int(mode)&ownerExecutionBitmask != 0
	executableByGroup := int(mode)&groupExecutableBitmask != 0
	executableByOthers := int(mode)&othersExecutableBitmask != 0

	return executableByOwner || executableByGroup || executableByOthers
}

//nolint:funlen,gocognit,gocyclo // cant take the walkDirFunc out, the signature is fixed and it writes to captured values outside
func (e *ExternalScriptsHandler) registerScriptPath(defaultScriptConfig *ConfigSection, conf *Config) error {
	configScriptPath, ok := defaultScriptConfig.GetString("script path")
	if !ok || configScriptPath == "" {
		return nil
	}

	// the script path may have '**' at the end, which toggles recursive search within that directory
	configScriptPath, recurseiveSearch := strings.CutSuffix(configScriptPath, "**")

	var scriptPath string
	var err error
	if scriptPath, err = filepath.Abs(configScriptPath); err != nil {
		return fmt.Errorf("could not get the absolute path from config script path %s : %w", configScriptPath, err)
	}

	stat, err := os.Stat(scriptPath)
	if os.IsNotExist(err) || !stat.IsDir() {
		return fmt.Errorf("script path %s: does not exist", scriptPath)
	}

	if !stat.IsDir() {
		return fmt.Errorf("script path %s: is not a directory", scriptPath)
	}

	foundScriptPaths := make([]string, 0)

	var walkDirFunc fs.WalkDirFunc

	walkDirFunc = func(path string, dirEntry fs.DirEntry, err error) error {
		// this type of function may have a non-nill error

		// First, if the initial Stat on the root directory fails, WalkDir calls the function with path set to root, d set to nil, and err set to the error from fs.Stat.
		if path == scriptPath && dirEntry == nil && err != nil {
			return err
		}

		// Second, if a directory's ReadDir method (see ReadDirFile) fails, WalkDir calls the function with path set to the directory's path,
		//  d set to an DirEntry describing the directory, and err set to the error from ReadDir.
		if dirEntry != nil && err != nil {
			return err
		}

		// cant use dirEntry, since symlink redirects recursive calls have dirEntry as nil
		// need to take os.stat for executable permissions
		stat, statErr := os.Stat(path)
		if os.IsNotExist(statErr) {
			return fmt.Errorf("path: %s, does not exist", path)
		}
		if statErr != nil {
			return fmt.Errorf("error when getting os.Stat of the path: %s , %w", path, statErr)
		}

		// if file lies outside of scriptPath, skip it.
		if !strings.HasPrefix(path, scriptPath) {
			log.Tracef("path : %s is outside of the scriptPath : %s", path, scriptPath)

			return nil
		}

		// if its an link, try to follow this link
		lstat, errLstat := os.Lstat(path)
		if errLstat == nil && (lstat.Mode().Type()&fs.ModeSymlink != 0) {
			log.Tracef("Following the symlink with name: %s at path: %s", lstat.Name(), path)

			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("reading the link failed, link path: %s", path)
			}

			if !filepath.IsAbs(linkTarget) {
				linkTarget, err = filepath.Abs(filepath.Join(filepath.Dir(path), linkTarget))

				if err != nil {
					return fmt.Errorf("error when converting the link target to absolute path: %s , linkTarget: %s", path, linkTarget)
				}
			}

			return walkDirFunc(linkTarget, nil, nil)
		}

		// optimization: if its a dir, return fs.SkipDir so that filepath.WalkDir can skip over the whole dir
		if !recurseiveSearch && stat.IsDir() && path != scriptPath {
			return fs.SkipDir
		}

		// Skip directories during the walk
		if stat.IsDir() {
			log.Tracef("skipping directory as a script to add: %s", path)

			return nil
		}

		if filepath.Dir(path) != scriptPath && !recurseiveSearch {
			log.Tracef("skipping file as a script to add: %s, recursiveSearch is not toggled", path)

			return nil
		}

		// They have to be regular files.
		if stat.Mode().Type()&fs.ModeIrregular != 0 {
			log.Tracef("skipping file as a script to add: %s, its an irregular file", path)

			return nil
		}

		// They have to be executable, available on some platforms
		checkExecutableRuntimes := [5]string{"linux", "darwin", "netbsd", "openbsd"}
		if slices.Contains(checkExecutableRuntimes[:], runtime.GOOS) {
			if !isExecutable(stat.Mode().Perm()) {
				log.Tracef("skipping file as a script to add: %s, go runtime is: %s, and the file is not executalbe with permissions: %d", path, runtime.GOOS, stat.Mode().Perm())

				return nil
			}
		} else {
			log.Tracef("file as a script to add: %s, go runtime is: %s, cannot determine if file is executable", path, runtime.GOOS)
		}

		log.Tracef("adding file as a script: %s", path)
		foundScriptPaths = append(foundScriptPaths, path)

		return nil
	}

	if err = filepath.WalkDir(scriptPath, walkDirFunc); err != nil {
		return fmt.Errorf("error when walking directory: %w", err)
	}

	for _, scriptPath := range foundScriptPaths {
		name := filepath.Base(scriptPath)
		cmdConf := conf.Section("/settings/external scripts/scripts/" + name)
		if !cmdConf.HasKey("command") {
			allow, _, _ := defaultScriptConfig.GetBool("allow arguments")
			if allow {
				cmdConf.Set("command", scriptPath+" %ARGS\"%")
			} else {
				cmdConf.Set("command", scriptPath)
			}
		}
	}

	return nil
}
