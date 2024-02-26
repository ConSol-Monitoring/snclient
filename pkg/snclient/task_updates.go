package snclient

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"pkg/utils"

	"github.com/sassoftware/go-rpmutils"
)

const (
	// Delay until the first check after a start is done
	UpdateCheckIntervalInitial = 2 * time.Second

	// Usual check interval
	UpdateCheckIntervalRegular = 55 * time.Second

	// Maximum file size for updates (prevent tar bombs)
	UpdateFileMaxSize = 100e6
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
	channel          string
	preRelease       bool
	updateInterval   float64
	updateHours      []UpdateHours
	updateDays       []UpdateDays

	httpOptions  *HTTPClientOptions
	lastUpdate   *time.Time
	lastModified map[string]*time.Time
}

type updatesAvailable struct {
	channel string
	url     string
	version string
	header  map[string]string
}

func NewUpdateHandler() Module {
	return &UpdateHandler{
		lastModified: make(map[string]*time.Time),
	}
}

func (u *UpdateHandler) Defaults() ConfigData {
	defaults := ConfigData{
		"automatic updates": "disabled",
		"automatic restart": "disabled",
		"channel":           "stable",
		"pre release":       "false",
		"update interval":   "1h",
		"update hours":      "0-24",
		"update days":       "mon-sun",
	}

	defaults.Merge(DefaultHTTPClientConfig)

	return defaults
}

func (u *UpdateHandler) Init(snc *Agent, section *ConfigSection, _ *Config, _ *ModuleSet) error {
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
	if channel, ok := section.GetString("channel"); ok {
		u.channel = channel
	}

	preRelease, ok, err := section.GetBool("pre release")
	switch {
	case err != nil:
		return fmt.Errorf("pre release: %s", err.Error())
	case ok:
		u.preRelease = preRelease
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
			_, err := u.CheckUpdates(false, true, u.automaticRestart, u.preRelease, "", u.channel, false)
			if err != nil {
				log.Errorf("[updates] checking for updates failed: %s", err.Error())
			}

			continue
		}
	}
}

func (u *UpdateHandler) CheckUpdates(force, download, restarts, preRelease bool, downgrade, channel string, forceUpdate bool) (version string, err error) {
	if !force {
		if !u.updatePreChecks() {
			return "", nil
		}
	}

	// channel might be a local file as well
	updateFile := ""
	best := &updatesAvailable{}
	_, err = os.ReadFile(channel)
	if err == nil {
		updateFile = channel
		best = &updatesAvailable{
			channel: "file",
			url:     "file://" + updateFile,
		}
	}

	// print options summary
	log.Tracef("[updates] starting update check")
	if updateFile != "" {
		log.Tracef("[updates] from file:    %s", updateFile)
	} else {
		log.Tracef("[updates] channel:      %s", channel)
	}
	log.Tracef("[updates] download:     %v", download)
	log.Tracef("[updates] auto restart: %v", restarts)
	if downgrade != "" {
		log.Tracef("[updates] downgrade:    yes: %s", downgrade)
	} else {
		log.Tracef("[updates] downgrade:    no")
	}

	now := time.Now()
	u.lastUpdate = &now

	// check for updates unless file specified
	if updateFile == "" {
		available := u.fetchAvailableUpdates(preRelease, channel)
		if len(available) == 0 {
			return "", nil
		}

		best = u.chooseBestUpdate(available, downgrade, forceUpdate)
		if best == nil {
			return "", nil
		}

		if !download {
			return best.version, nil
		}
	}

	return u.finishUpdateCheck(best, restarts)
}

func (u *UpdateHandler) finishUpdateCheck(best *updatesAvailable, restarts bool) (version string, err error) {
	updateFile, err := u.downloadUpdate(best)
	if err != nil {
		return "", err
	}

	newVersion, err := u.verifyUpdate(updateFile)
	if err != nil {
		LogError(os.Remove(updateFile))

		return "", err
	}

	if utils.ParseVersion(newVersion) < utils.ParseVersion(u.snc.Version()) {
		log.Warnf("[update] downgrading to %s", newVersion)
	}

	if restarts {
		log.Infof("[update] update successful from %s to %s, restarting into new version", u.snc.Version(), newVersion)
		err = u.ApplyRestart(updateFile)
		if err != nil {
			return "", err
		}
	} else {
		log.Infof("[update] version %s successfully downloaded: %s", newVersion, updateFile)
		err = u.Apply(updateFile)
		if err != nil {
			return "", err
		}
	}

	return newVersion, nil
}

func (u *UpdateHandler) chooseBestUpdate(updates []updatesAvailable, downgrade string, forceUpdate bool) (best *updatesAvailable) {
	down := float64(-1)
	if downgrade != "" {
		down = utils.ParseVersion(downgrade)
	}

	bestVersion := float64(0)
	for num, u := range updates {
		version := utils.ParseVersion(u.version)
		if down != -1 {
			if version == down {
				return &updates[num]
			}

			continue
		}
		if best == nil || version > bestVersion {
			best = &updates[num]
			bestVersion = version
		}
	}

	if down != -1 {
		log.Warnf("did not find requested version (%s) to downgrade to:", downgrade)
		for _, u := range updates {
			log.Warnf("  - %s (from %s)", u.version, u.url)
		}
	}

	curVersion := utils.ParseVersion(u.snc.Version())
	if bestVersion <= curVersion {
		if forceUpdate {
			return best
		}
		return nil
	}

	return best
}

func (u *UpdateHandler) fetchAvailableUpdates(preRelease bool, channel string) (updates []updatesAvailable) {
	available := []updatesAvailable{}
	channelConfSection := u.snc.Config.Section("/settings/updates/channel")
	if channel == "all" {
		channel = strings.Join(channelConfSection.Keys(), ",")
	}
	chanList := strings.Split(channel, ",")
	for _, channel := range chanList {
		channel = strings.TrimSpace(channel)
		if channel == "" {
			continue
		}
		url, ok := channelConfSection.GetString(channel)
		if !ok {
			log.Warnf("no update channel '%s', check the %s config section.", channel, channelConfSection.name)
			log.Infof("available channel: %s", strings.Join(u.getAvailableChannel(), ", "))

			continue
		}

		log.Tracef("next: %s channel: %s", channel, url)

		updates, err := u.checkUpdate(url, preRelease, channel)
		if err != nil {
			log.Warnf("channel %s failed: %s", channel, err.Error())

			continue
		}

		available = append(available, updates...)
	}

	return available
}

func (u *UpdateHandler) checkUpdate(url string, preRelease bool, channel string) (updates []updatesAvailable, err error) {
	if ok, _ := regexp.MatchString(`^https://api\.github\.com/repos/.*/releases`, url); ok {
		updates, err = u.checkUpdateGithubRelease(url, preRelease)
	} else if ok, _ := regexp.MatchString(`^https://api\.github\.com/repos/.*/actions/artifacts`, url); ok {
		updates, err = u.checkUpdateGithubActions(url, channel)
	} else if ok, _ := regexp.MatchString(`^file:`, url); ok {
		updates, err = u.checkUpdateFile(url)
	} else {
		updates, err = u.checkUpdateCustomURL(url)
	}

	if err != nil {
		return nil, err
	}

	log.Debugf("found %d versions in %s channel:", len(updates), channel)
	for i, u := range updates {
		updates[i].channel = channel
		log.Debugf("  - %s (from %s)", u.version, u.url)
	}

	return updates, nil
}

// check available updates from github release page
func (u *UpdateHandler) checkUpdateGithubRelease(url string, preRelease bool) (updates []updatesAvailable, err error) {
	log.Tracef("[update] checking github release url at: %s", url)
	resp, err := u.snc.httpDo(*u.ctx, u.httpOptions, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("http: %s", err.Error())
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
		return nil, fmt.Errorf("json: %s", err.Error())
	}

	if len(releases) == 0 {
		log.Debugf("[update] no releases found")

		return nil, nil
	}

	for i := range releases {
		release := releases[i]
		if release.PreRelease && !preRelease {
			log.Debugf("skipping pre release: %s", release.TagName)

			continue
		}

		log.Debugf("checking assets for release: %s", release.TagName)
		foundOne := false
		for _, asset := range release.Assets {
			if u.isUsableGithubAsset(strings.ToLower(asset.Name)) {
				updates = append(updates, updatesAvailable{url: asset.URL, version: release.TagName})
				foundOne = true
			}
		}
		if !foundOne {
			log.Debugf("[update] no download url for this architecture found: os:%s arch:%s", runtime.GOOS, runtime.GOARCH)
		}
	}

	return updates, nil
}

// check available updates from github actions page
func (u *UpdateHandler) checkUpdateGithubActions(url, channel string) (updates []updatesAvailable, err error) {
	log.Tracef("[update] checking github action url at: %s", url)
	conf := u.snc.Config.Section("/settings/updates/channel/" + channel)
	token, ok := conf.GetString("github token")
	if !ok || token == "" || token == "<GITHUB-TOKEN>" { //nolint:gosec // false positive token, this is no token
		return nil, fmt.Errorf("github action urls require a github token to work, skipping")
	}
	header := map[string]string{
		"Authorization": "Bearer " + token,
	}
	// show some more than the default 30, 100 seems to be maximum
	resp, err := u.snc.httpDo(*u.ctx, u.httpOptions, "GET", url+"?per_page=100", header)
	if err != nil {
		return nil, fmt.Errorf("http: %s", err.Error())
	}
	defer resp.Body.Close()

	type GithubArtifact struct {
		URL  string `json:"archive_download_url"`
		Name string `json:"name"`
	}

	type GithubActions struct {
		Artifacts []GithubArtifact `json:"artifacts"`
	}
	var artifacts GithubActions
	err = json.NewDecoder(resp.Body).Decode(&artifacts)
	if err != nil {
		return nil, fmt.Errorf("json: %s", err.Error())
	}

	log.Debugf("[update] found %d action artifacts in %s channel", len(artifacts.Artifacts), channel)
	if len(artifacts.Artifacts) == 0 {
		return nil, nil
	}

	reActionVersion := regexp.MustCompile(`^snclient\-(.*?)\-\w+-\w+\.\w+`)

	for i := range artifacts.Artifacts {
		artifact := artifacts.Artifacts[i]
		if u.isUsableGithubAsset(strings.ToLower(artifact.Name)) {
			matches := reActionVersion.FindStringSubmatch(artifact.Name)
			if len(matches) > 1 {
				version := matches[1]
				updates = append(updates, updatesAvailable{url: artifact.URL, version: version, header: header})
			}
		}
	}

	if len(updates) == 0 {
		log.Debugf("[update] no matching artifacts url for this architecture found: os:%s arch:%s", runtime.GOARCH, runtime.GOOS)
	}

	return updates, nil
}

// check available update from any url
func (u *UpdateHandler) checkUpdateCustomURL(url string) (updates []updatesAvailable, err error) {
	log.Tracef("[update] checking custom url at: %s", url)
	resp, err := u.snc.httpDo(*u.ctx, u.httpOptions, "HEAD", url, nil)
	if err != nil {
		return nil, fmt.Errorf("http: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.ContentLength < 0 {
		return nil, fmt.Errorf("request failed %s: got content length %d", url, resp.ContentLength)
	}

	executable := GlobalMacros["exe-full"]
	stat, err := os.Stat(executable)
	if err != nil {
		return nil, fmt.Errorf("stat: %s", err.Error())
	}

	if resp.ContentLength > 0 && resp.ContentLength != stat.Size() {
		log.Tracef("[update] content size differs %s: %d vs. %s: %d", url, resp.ContentLength, executable, stat.Size())

		return []updatesAvailable{{url: url, version: ""}}, nil
	}

	lastModified := resp.Header.Get("Last-Modified")
	if lastModified != "" {
		modifiedTime, err := time.Parse(http.TimeFormat, lastModified)
		if err != nil {
			log.Debugf("error parsing Last-Modified header: %s", err)
		} else {
			prev, ok := u.lastModified[url]
			if ok && prev.Before(modifiedTime) {
				log.Tracef("[update] last-modified differs for %s", url)
				log.Tracef("[update] old %s", modifiedTime.UTC().String())
				log.Tracef("[update] new %s", u.lastUpdate.UTC().String())

				return []updatesAvailable{{url: url, version: ""}}, nil
			}
			u.lastModified[url] = &modifiedTime
		}
	}

	log.Tracef("[update] no update available, %s matches the last version from %s.", executable, url)

	return []updatesAvailable{{url: url, version: ""}}, nil
}

// check available update from local or remote filesystem
func (u *UpdateHandler) checkUpdateFile(url string) (updates []updatesAvailable, err error) {
	localPath := strings.TrimPrefix(url, "file://")
	log.Tracef("[update] checking local file at: %s", localPath)
	_, err = os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("could not find update file: %s", err.Error())
	}

	// copy to tmp location
	tempFile, err := os.CreateTemp("", "snclient-tmpupdate")
	if err != nil {
		return nil, fmt.Errorf("mktemp: %s", err.Error())
	}
	LogError(tempFile.Close())
	os.Remove(tempFile.Name())
	tempUpdate := tempFile.Name() + GlobalMacros["file-ext"]
	err = utils.CopyFile(localPath, tempUpdate)
	if err != nil {
		return nil, fmt.Errorf("copy update file failed: %s", err.Error())
	}

	err = u.extractUpdate(tempUpdate)
	if err != nil {
		return nil, fmt.Errorf("extracting update failed: %s", err.Error())
	}

	// get version from that executable
	version, err := u.verifyUpdate(tempUpdate)
	if err != nil {
		return nil, err
	}

	return []updatesAvailable{{url: url, version: version}}, nil
}

// fetch update file into tmp file
func (u *UpdateHandler) downloadUpdate(update *updatesAvailable) (binPath string, err error) {
	url := update.url
	var src io.ReadCloser
	if strings.HasPrefix(url, "file://") {
		localPath := strings.TrimPrefix(url, "file://")
		log.Tracef("[update] fetching update from %s", localPath)
		file, err2 := os.Open(localPath)
		if err2 != nil {
			return "", fmt.Errorf("open failed %s: %s", localPath, err2.Error())
		}
		src = file
	} else {
		log.Tracef("[update] downloading update from %s", url)
		resp, err2 := u.snc.httpDo(*u.ctx, u.httpOptions, "GET", url, update.header)
		if err2 != nil {
			return "", fmt.Errorf("fetching update failed %s: %s", url, err2.Error())
		}
		defer resp.Body.Close()
		src = resp.Body
	}

	executable := GlobalMacros["exe-full"]
	updateFile := u.snc.buildUpdateFile(executable)
	saveFile, err := os.Create(updateFile)
	if err != nil {
		return "", fmt.Errorf("open: %s", err.Error())
	}

	log.Tracef("[update] saving to %s", saveFile.Name())

	_, err = io.Copy(saveFile, src)
	if err != nil {
		saveFile.Close()

		return "", fmt.Errorf("read: %s", err.Error())
	}
	saveFile.Close()

	err = u.extractUpdate(updateFile)
	if err != nil {
		return "", err
	}

	return updateFile, nil
}

func (u *UpdateHandler) extractUpdate(updateFile string) (err error) {
	executable := GlobalMacros["exe-full"]

	// what file type did we download?
	mime, err := utils.MimeType(updateFile)
	if err != nil {
		return fmt.Errorf("mime: %s", err.Error())
	}

	startOver := true
	log.Tracef("detected mime %s on downloaded file %s", mime, updateFile)
	switch mime {
	case "application/zip":
		err = u.extractZip(updateFile)
	case "application/x-gzip":
		err = u.extractGZip(updateFile)
	case "application/x-tar":
		err = u.extractTar(updateFile)
	case "application/rpm":
		err = u.extractRpm(updateFile)
	case "application/msi":
		err = u.extractMsi(updateFile)
	case "application/xar":
		err = u.extractXar(updateFile)
	default:
		startOver = false
	}

	if startOver {
		if err != nil {
			return err
		}
		LogError(utils.CopyFileMode(executable, updateFile))

		return u.extractUpdate(updateFile)
	}

	err = utils.CopyFileMode(executable, updateFile)
	if err != nil {
		return fmt.Errorf("chmod %s: %s", updateFile, err.Error())
	}

	return nil
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

func (u *UpdateHandler) ApplyRestart(bin string) error {
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

func (u *UpdateHandler) Apply(bin string) error {
	cmd := exec.Command(bin, "update")
	cmd.Env = os.Environ()

	if IsInteractive() || u.snc.flags.Mode == ModeOneShot {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Args = append(cmd.Args, "--logfile=stderr", "apply")
	}
	log.Tracef("[update] start updated file %s %s", cmd.Path, strings.Join(cmd.Args[1:], " "))
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("starting updater failed: %s", err.Error())
	}

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

	executable := GlobalMacros["exe-full"]
	if strings.Contains(executable, ".update") {
		log.Tracef("[updates] started from a tmp update file, skipping")

		return false
	}

	return true
}

func (u *UpdateHandler) extractZip(fileName string) error {
	zipHandle, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("zip: %s", err.Error())
	}
	defer zipHandle.Close()

	if len(zipHandle.File) != 1 {
		return fmt.Errorf("expect zip must contain exactly one file, have: %d", len(zipHandle.File))
	}

	tempFile, err := os.CreateTemp("", "snclient-unzip")
	if err != nil {
		return fmt.Errorf("mktemp: %s", err.Error())
	}

	src, err := zipHandle.File[0].Open()
	if err != nil {
		return fmt.Errorf("zip open: %s", err.Error())
	}
	defer src.Close()

	_, err = io.Copy(tempFile, src)
	if err != nil {
		tempFile.Close()

		return fmt.Errorf("read: %s", err.Error())
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	log.Tracef("cp %s %s", tempFile.Name(), fileName)
	err = utils.CopyFile(tempFile.Name(), fileName)
	if err != nil {
		return fmt.Errorf("cp: %s", err.Error())
	}

	return nil
}

func (u *UpdateHandler) extractGZip(fileName string) error {
	srcFile, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("open: %s", err.Error())
	}
	defer srcFile.Close()
	gzipHandle, err := gzip.NewReader(srcFile)
	if err != nil {
		return fmt.Errorf("gzip: %s", err.Error())
	}
	defer gzipHandle.Close()

	tempFile, err := os.CreateTemp("", "snclient-gunzip")
	if err != nil {
		return fmt.Errorf("mktemp: %s", err.Error())
	}

	_, err = io.CopyN(tempFile, gzipHandle, UpdateFileMaxSize)
	if err != nil && !errors.Is(err, io.EOF) {
		tempFile.Close()

		return fmt.Errorf("read: %s", err.Error())
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	log.Tracef("cp %s %s", tempFile.Name(), fileName)
	err = utils.CopyFile(tempFile.Name(), fileName)
	if err != nil {
		return fmt.Errorf("cp: %s", err.Error())
	}

	return nil
}

func (u *UpdateHandler) extractTar(fileName string) error {
	tarFile, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("tar open: %s", err.Error())
	}
	defer tarFile.Close()

	tempFile, err := os.CreateTemp("", "snclient-tar")
	if err != nil {
		return fmt.Errorf("mktemp: %s", err.Error())
	}
	defer tempFile.Close()

	found := false
	tarHandle := tar.NewReader(tarFile)
	for {
		hdr, err2 := tarHandle.Next()
		if errors.Is(err2, io.EOF) {
			break
		}
		if err2 != nil {
			return fmt.Errorf("tar read: %s", err2.Error())
		}
		if found {
			return fmt.Errorf("tarball must contain only one file, got another: %s", hdr.Name)
		}

		log.Tracef("copying %s from tarball", hdr.Name)
		if _, err = io.CopyN(tempFile, tarHandle, UpdateFileMaxSize); err != nil {
			if !errors.Is(err, io.EOF) {
				return fmt.Errorf("tar read: %s", err.Error())
			}
		}
		tempFile.Close()
		found = true
	}

	if !found {
		return fmt.Errorf("did not find snclient binary in tar file")
	}

	log.Tracef("cp %s %s", tempFile.Name(), fileName)
	err = utils.CopyFile(tempFile.Name(), fileName)
	if err != nil {
		return fmt.Errorf("cp: %s", err.Error())
	}

	return nil
}

func (u *UpdateHandler) extractRpm(fileName string) error {
	rpmFile, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("rpm open: %s", err.Error())
	}
	defer rpmFile.Close()

	rpm, err := rpmutils.ReadRpm(rpmFile)
	if err != nil {
		return fmt.Errorf("read rpm: %s", err.Error())
	}

	tempDir, err := os.MkdirTemp("", "snclient-tmprpm")
	if err != nil {
		return fmt.Errorf("MkdirTemp: %s", err.Error())
	}
	defer os.RemoveAll(tempDir)

	// Extracting payload
	if err = rpm.ExpandPayload(tempDir); err != nil {
		return fmt.Errorf("rpm unpack: %s", err.Error())
	}

	log.Tracef("cp %s %s", path.Join(tempDir, "/usr/bin/snclient"), fileName)
	err = utils.CopyFile(path.Join(tempDir, "/usr/bin/snclient"), fileName)
	if err != nil {
		return fmt.Errorf("mv: %s", err.Error())
	}

	return nil
}

func (u *UpdateHandler) extractMsi(fileName string) error {
	// Create a temporary directory to extract the contents of the .msi file
	tempDir, err := os.MkdirTemp("", "snclient-tmpmsi")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %s", err.Error())
	}
	defer os.RemoveAll(tempDir)
	log.Tracef("temp dir: %s", tempDir)

	// Use the "msiexec" command to extract the file from the .msi
	cmd := exec.Command("msiexec", "/a", fileName, "/qn", "TARGETDIR="+tempDir) //nolint:gosec // no user input here
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("failed to run msiexec %s: %s", strings.Join(cmd.Args, " "), err.Error())
	}

	extractedFilePath := ""
	err = filepath.Walk(tempDir, func(path string, _ os.FileInfo, _ error) error {
		if strings.HasSuffix(path, "snclient.exe") {
			extractedFilePath = path
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("filewalk: %s", err.Error())
	}

	if extractedFilePath == "" {
		return fmt.Errorf("did not find snclient.exe in msi file")
	}

	log.Tracef("cp %s %s", extractedFilePath, fileName)
	err = utils.CopyFile(extractedFilePath, fileName)
	if err != nil {
		return fmt.Errorf("cp: %s", err.Error())
	}

	return nil
}

func (u *UpdateHandler) extractXar(fileName string) error {
	// Create a temporary directory to extract the contents of the .pkg file
	tempDir, err := os.MkdirTemp("", "snclient-tmpxar")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %s", err.Error())
	}
	defer os.RemoveAll(tempDir)
	log.Tracef("temp dir: %s", tempDir)

	// Use the "xar" command to extract the file from the .pkg
	cmd := exec.Command("xar", "-xf", fileName)
	cmd.Dir = tempDir
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("failed to run xar %s: %s", strings.Join(cmd.Args, " "), err.Error())
	}

	// Unpack Payload from the .pkg
	cmd = exec.Command("/bin/sh", "-c", "cat Payload | gunzip -dc |cpio -i")
	cmd.Dir = tempDir
	if err2 := cmd.Run(); err2 != nil {
		return fmt.Errorf("failed to unpack %s: %s", strings.Join(cmd.Args, " "), err2.Error())
	}

	extractedFilePath := ""
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, "snclient") {
			extractedFilePath = path
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("filewalk: %s", err.Error())
	}

	if extractedFilePath == "" {
		return fmt.Errorf("did not find snclient binary in the pkg file")
	}

	log.Tracef("cp %s %s", extractedFilePath, fileName)
	err = utils.CopyFile(extractedFilePath, fileName)
	if err != nil {
		return fmt.Errorf("cp: %s", err.Error())
	}

	return nil
}

func (u *UpdateHandler) isUsableGithubAsset(name string) bool {
	archVariants := []string{runtime.GOARCH}
	switch runtime.GOARCH {
	case "386":
		archVariants = append(archVariants, "i386")
	case "arm64":
		archVariants = append(archVariants, "aarch64")
	}

	osVariants := []string{runtime.GOOS}
	switch runtime.GOOS {
	case "darwin":
		osVariants = append(osVariants, "osx")
	case "windows":
		if runtime.GOARCH == "arm64" {
			// arm windows can use the 64bit version
			archVariants = append(archVariants, "amd64")
		}
	}

	for _, arch := range archVariants {
		for _, os := range osVariants {
			lookFor := strings.ToLower(fmt.Sprintf("%s-%s", os, arch))
			if strings.Contains(name, lookFor) {
				// right now we can only extract .rpm, .msi and .pkg
				if strings.Contains(name, ".rpm") || strings.Contains(name, ".msi") || strings.Contains(name, ".pkg") {
					return true
				}
				log.Tracef("skip: unusable package format: %s", name)
			}
		}
	}

	return false
}

func (u *UpdateHandler) getAvailableChannel() []string {
	channelConfSection := u.snc.Config.Section("/settings/updates/channel")
	available := channelConfSection.Keys()
	sort.Strings(available)

	return available
}
