package snclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"pkg/utils"
)

const (
	// Delay until the first check after a start is done
	UpdateCheckIntervalInitial = 2 * time.Second

	// Usual check interval
	UpdateCheckIntervalRegular = 55 * time.Second
)

var reVersion = regexp.MustCompile(`SNClient.*?\s+(v[\d.]+)\s+`)

func init() {
	RegisterModule(&AvailableTasks, "Updates", "/settings/updates", NewUpdateHandler)
}

type UpdateHandler struct {
	noCopy noCopy

	snc *Agent

	ctx    *context.Context
	cancel context.CancelFunc

	automaticUpdates bool
	automaticRestart bool
	updateURL        string
	updateInterval   float64
	updateHours      []UpdateHours
	updateDays       []UpdateDays

	httpOptions  *HTTPClientOptions
	lastUpdate   *time.Time
	lastModified *time.Time
}

func NewUpdateHandler() Module {
	return &UpdateHandler{}
}

func (u *UpdateHandler) Defaults() ConfigData {
	defaults := ConfigData{
		"automatic updates": "disabled",
		"automatic restart": "disabled",
		"update url":        "https://api.github.com/repos/ConSol-monitoring/snclient/releases",
		"update interval":   "1h",
		"update hours":      "0-24",
		"update days":       "mon-sun",
	}

	defaults.Merge(DefaultHTTPClientConfig)

	return defaults
}

func (u *UpdateHandler) Init(snc *Agent, section *ConfigSection, _ *Config) error {
	u.snc = snc
	ctx, cancel := context.WithCancel(context.Background())
	u.ctx = &ctx
	u.cancel = cancel

	httpOptions, err := snc.buildClientHTTPOptions(section)
	if err != nil {
		return err
	}
	u.httpOptions = httpOptions

	return u.setConfig(section)
}

func (u *UpdateHandler) setConfig(section *ConfigSection) error {
	if updateURL, ok := section.GetString("update url"); ok {
		u.updateURL = updateURL
	}

	autoUpdate, ok, err := section.GetBool("automatic updates")
	switch {
	case err != nil:
		return fmt.Errorf("automatic updates: %s", err.Error())
	case ok:
		u.automaticUpdates = autoUpdate
	}

	autoRestart, ok, err := section.GetBool("automatic restart")
	switch {
	case err != nil:
		return fmt.Errorf("automatic restarts: %s", err.Error())
	case ok:
		u.automaticRestart = autoRestart
	}

	updateInterval, ok, err := section.GetDuration("update interval")
	switch {
	case err != nil:
		return fmt.Errorf("update interval: %s", err.Error())
	case ok:
		u.updateInterval = updateInterval
	}

	if updateHours, ok := section.GetString("update hours"); ok {
		hours, err := NewUpdateHours(updateHours)
		if err != nil {
			return fmt.Errorf("update hours: %s", err.Error())
		}
		u.updateHours = hours
	}

	if updateDays, ok := section.GetString("update days"); ok {
		days, err := NewUpdateDays(updateDays)
		if err != nil {
			return fmt.Errorf("update days: %s", err.Error())
		}
		u.updateDays = days
	}

	return nil
}

func (u *UpdateHandler) Start() error {
	go u.mainLoop()

	return nil
}

func (u *UpdateHandler) Stop() {
	u.cancel()
}

func (u *UpdateHandler) mainLoop() {
	if !u.automaticUpdates {
		log.Debugf("[updates] automatic updates disabled, won't check for updates.")

		return
	}

	ticker := time.NewTicker(UpdateCheckIntervalInitial)
	defer ticker.Stop()

	interval := UpdateCheckIntervalRegular
	if interval > time.Duration(u.updateInterval) {
		interval = time.Duration(u.updateInterval) * time.Second
	}

	for {
		select {
		case <-(*u.ctx).Done():
			log.Tracef("[updates] stopping UpdateHandler mainLoop")

			return
		case <-ticker.C:
			ticker.Reset(interval)
			err := u.checkUpdate(false)
			if err != nil {
				log.Errorf("[updates] checking for updates failed: %s", err.Error())
			}

			continue
		}
	}
}

func (u *UpdateHandler) checkUpdate(force bool) (err error) {
	if !force {
		if !u.updatePreChecks() {
			return nil
		}
	}

	log.Tracef("[updates] starting update check")
	now := time.Now()
	u.lastUpdate = &now

	var downloadURL string
	if ok, _ := regexp.MatchString(`^https://api\.github\.com/repos/.*/releases`, u.updateURL); ok {
		downloadURL, err = u.checkUpdateGithubRelease()
	} else if ok, _ := regexp.MatchString(`^https://github\.com/.*/actions`, u.updateURL); ok {
		downloadURL, err = u.checkUpdateGithubActions()
	} else if ok, _ := regexp.MatchString(`^file:`, u.updateURL); ok {
		downloadURL, err = u.checkUpdateFile()
	} else {
		downloadURL, err = u.checkUpdateCustomURL()
	}

	if err != nil {
		return err
	}

	if downloadURL == "" {
		return nil
	}

	updateFile, err := u.downloadUpdate(downloadURL)
	if err != nil {
		return err
	}

	newVersion, err := u.verifyUpdate(updateFile)
	if err != nil {
		LogError(os.Remove(updateFile))

		return err
	}

	if u.automaticRestart {
		if utils.ParseVersion(newVersion) < utils.ParseVersion(u.snc.Version()) {
			log.Warnf("[update] downgrading to %s", newVersion)
		}
		log.Infof("[update] update successful from %s to %s, restarting into new version", u.snc.Version(), newVersion)
		err = u.applyRestart(updateFile)
		if err != nil {
			return err
		}
	} else {
		log.Infof("[update] update to version %s successful (no automatic restart)", newVersion)
	}

	return nil
}

// check available updates from github release page
func (u *UpdateHandler) checkUpdateGithubRelease() (downloadURL string, err error) {
	resp, err := u.snc.httpDo(*u.ctx, u.httpOptions, "GET", u.updateURL, nil)
	if err != nil {
		return "", fmt.Errorf("http: %s", err.Error())
	}
	defer resp.Body.Close()

	type GithubAsset struct {
		URL  string `json:"browser_download_url"`
		Name string `json:"name"`
	}

	type GithubRelease struct {
		Name       string        `json:"name"`
		Draft      bool          `json:"draft"`
		PreRelease bool          `json:"prerelease"`
		TagName    string        `json:"tag_name"`
		Assets     []GithubAsset `json:"assets"`
	}
	var releases []GithubRelease
	err = json.NewDecoder(resp.Body).Decode(&releases)
	if err != nil {
		return "", fmt.Errorf("json: %s", err.Error())
	}

	lastVersion := float64(0)
	var lastRelease *GithubRelease
	for i := range releases {
		release := releases[i]
		vers := utils.ParseVersion(release.TagName)
		if vers > lastVersion {
			lastVersion = utils.ParseVersion(release.TagName)
			lastRelease = &release
		}
	}
	if lastRelease == nil {
		log.Debugf("[update] no releases found")

		return "", nil
	}

	if lastVersion <= utils.ParseVersion(VERSION) {
		log.Debugf("[update] no updates found, last github release is: %s", lastRelease.TagName)

		return "", nil
	}

	archVariants := []string{runtime.GOARCH}
	switch runtime.GOARCH {
	case "386":
		archVariants = append(archVariants, "i386")
	case "arm64":
		archVariants = append(archVariants, "aarch64")
	}
	for _, arch := range archVariants {
		lookFor := strings.ToLower(fmt.Sprintf("%s-%s", runtime.GOOS, arch))
		for _, asset := range lastRelease.Assets {
			if strings.Contains(strings.ToLower(asset.Name), "bin-") && strings.Contains(strings.ToLower(asset.Name), lookFor) {
				return asset.URL, nil
			}
		}
	}

	log.Debugf("[update] no download url for this architecture found: os:%s arch:%s", runtime.GOARCH, runtime.GOOS)

	return "", nil
}

// check available updates from github actions page
func (u *UpdateHandler) checkUpdateGithubActions() (downloadURL string, err error) {
	return "", nil
}

// check available update from any url
func (u *UpdateHandler) checkUpdateCustomURL() (downloadURL string, err error) {
	resp, err := u.snc.httpDo(*u.ctx, u.httpOptions, "HEAD", u.updateURL, nil)
	if err != nil {
		return "", fmt.Errorf("http: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.ContentLength < 0 {
		return "", fmt.Errorf("request failed %s: got content length %d", u.updateURL, resp.ContentLength)
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not detect path to executable: %s", err.Error())
	}

	stat, err := os.Stat(executable)
	if err != nil {
		return "", fmt.Errorf("stat: %s", err.Error())
	}

	if resp.ContentLength > 0 && resp.ContentLength != stat.Size() {
		log.Tracef("[update] content size differs %s: %d vs. %s: %d", u.updateURL, resp.ContentLength, executable, stat.Size())

		return u.updateURL, nil
	}

	lastModified := resp.Header.Get("Last-Modified")
	if lastModified != "" {
		modifiedTime, err := time.Parse(http.TimeFormat, lastModified)
		if err != nil {
			log.Debugf("error parsing Last-Modified header: %s", err)
		} else {
			if u.lastModified != nil && u.lastModified.Before(modifiedTime) {
				log.Tracef("[update] last-modified differs for %s", u.updateURL)
				log.Tracef("[update] old %s", modifiedTime.UTC().String())
				log.Tracef("[update] new %s", u.lastUpdate.UTC().String())

				return u.updateURL, nil
			}
			u.lastModified = &modifiedTime
		}
	}

	log.Tracef("[update] no update available, %s matches the last version from %s.", executable, u.updateURL)

	return "", nil
}

// check available update from local filesystem
func (u *UpdateHandler) checkUpdateFile() (downloadURL string, err error) {
	localPath := strings.TrimPrefix(u.updateURL, "file://")
	stat, err := os.Stat(localPath)
	if err != nil {
		return "", fmt.Errorf("could not find update file: %s", err.Error())
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not detect path to executable: %s", err.Error())
	}

	oldStat, err := os.Stat(executable)
	if err != nil {
		return "", fmt.Errorf("stat: %s", err.Error())
	}
	if oldStat.Size() != stat.Size() {
		log.Tracef("[update] size differs %s: %d vs. %s: %d", localPath, stat.Size(), executable, oldStat.Size())

		return u.updateURL, nil
	}

	sum1, err := utils.Sha256Sum(localPath)
	if err != nil {
		return "", fmt.Errorf("sha256sum %s: %s", localPath, err.Error())
	}

	sum2, err := utils.Sha256Sum(executable)
	if err != nil {
		return "", fmt.Errorf("sha256sum %s: %s", executable, err.Error())
	}

	if sum1 != sum2 {
		log.Tracef("[update] checksum differs %s: %s vs. %s: %s", localPath, sum1, executable, sum2)

		return u.updateURL, nil
	}

	log.Tracef("[update] no update available, %s matches the last version at %s.", executable, localPath)

	return "", nil
}

// fetch update file into tmp file
func (u *UpdateHandler) downloadUpdate(url string) (binPath string, err error) {
	var src io.ReadCloser
	if strings.HasPrefix(url, "file://") {
		localPath := strings.TrimPrefix(u.updateURL, "file://")
		log.Tracef("[update] fetching update from %s", localPath)
		file, err := os.Open(localPath)
		if err != nil {
			return "", fmt.Errorf("open failed %s: %s", localPath, err.Error())
		}
		src = file
	} else {
		log.Tracef("[update] downloading update from %s", url)
		resp, err := u.snc.httpDo(*u.ctx, u.httpOptions, "GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("fetching update failed %s: %s", url, err.Error())
		}
		defer resp.Body.Close()
		src = resp.Body
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not detect path to executable: %s", err.Error())
	}
	updateFile := u.snc.buildUpdateFile(executable)
	tmpFile, err := os.Create(updateFile)
	if err != nil {
		return "", fmt.Errorf("open: %s", err.Error())
	}

	log.Tracef("[update] saving to %s", tmpFile.Name())
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, src)
	if err != nil {
		tmpFile.Close()

		return "", fmt.Errorf("read: %s", err.Error())
	}

	err = utils.CopyFileMode(executable, updateFile)
	if err != nil {
		return "", fmt.Errorf("chmod %s: %s", updateFile, err.Error())
	}

	return updateFile, nil
}

func (u *UpdateHandler) verifyUpdate(newBinPath string) (version string, err error) {
	log.Tracef("[update] checking update file %s", newBinPath)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, newBinPath, "-V")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("checking new version failed %s: %s", newBinPath, err.Error())
	}
	log.Tracef("[update] %s -V: %s", newBinPath, strings.TrimSpace(string(output)))
	if matches := reVersion.FindStringSubmatch(string(output)); len(matches) > 0 {
		version = matches[1]
	} else {
		return "", fmt.Errorf("could not extract version from updated binary: %s", output)
	}

	return version, nil
}

func (u *UpdateHandler) applyRestart(bin string) error {
	u.snc.stop()
	log.Tracef("[update] re-exec into new file %s %v", bin, os.Args[1:])
	if runtime.GOOS == "windows" {
		// cannot re-exec on windows, need to start a separate updater
		cmd := exec.Cmd{
			Path: bin,
			Args: os.Args,
			Env:  os.Environ(),
		}
		err := cmd.Start()
		if err != nil {
			return fmt.Errorf("starting updater failed: %s", err.Error())
		}
	} else {
		err := syscall.Exec(bin, os.Args, os.Environ()) //nolint:gosec // false positive? There should be no tainted input here
		if err != nil {
			return fmt.Errorf("restart failed: %s", err.Error())
		}
	}

	u.snc.CleanExit(ExitCodeOK)

	return nil
}

func (u *UpdateHandler) updatePreChecks() bool {
	if !u.automaticUpdates {
		return false
	}

	if u.lastUpdate != nil {
		if u.lastUpdate.After(time.Now().Add(time.Duration(-u.updateInterval) * time.Second)) {
			if log.IsV(4) {
				log.Tracef("[updates] no update check required, last check: %s", u.lastUpdate.String())
			}

			return false
		}
	}

	if len(u.updateHours) > 0 {
		inTime := false
		now := time.Now()
		for _, hour := range u.updateHours {
			if hour.InTime(now) {
				inTime = true

				break
			}
		}

		if !inTime {
			log.Tracef("[updates] skipping check, not in update hours time period")

			return false
		}
	}

	if len(u.updateDays) > 0 {
		inTime := false
		now := time.Now()
		for _, day := range u.updateDays {
			if day.InTime(now) {
				inTime = true

				break
			}
		}

		if !inTime {
			log.Tracef("[updates] skipping check, not in update days time period")

			return false
		}
	}

	executable, err := os.Executable()
	if err != nil {
		log.Tracef("could not detect path to executable: %s", err.Error())

		return false
	}
	if strings.Contains(executable, ".update") {
		log.Tracef("[updates] started from a tmp update file, skipping")

		return false
	}

	return true
}
