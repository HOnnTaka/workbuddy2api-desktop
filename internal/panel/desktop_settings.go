// desktop_settings.go 桌面端特定配置：开机自启、最小化静默启动管理。
package panel

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type DesktopSettings struct {
	AutoStart      bool `json:"auto_start"`
	StartMinimized bool `json:"start_minimized"`
}

func defaultDesktopSettings() DesktopSettings {
	return DesktopSettings{
		AutoStart:      false,
		StartMinimized: true,
	}
}

func getDesktopSettingsFilePath() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		return filepath.Join(dir, "desktop_settings.json")
	}
	return "desktop_settings.json"
}

func loadDesktopSettings() DesktopSettings {
	path := getDesktopSettingsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultDesktopSettings()
	}
	var s DesktopSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultDesktopSettings()
	}
	return s
}

func saveDesktopSettingsToFile(s DesktopSettings) error {
	path := getDesktopSettingsFilePath()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func setWindowsAutoStart(enable bool) {
	if runtime.GOOS != "windows" {
		return
	}
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exePath)
	desktopExe := filepath.Join(dir, "wb2api-panel-desktop.exe")
	if _, err := os.Stat(desktopExe); err != nil {
		if _, err := os.Stat("wb2api-panel-desktop.exe"); err == nil {
			desktopExe, _ = filepath.Abs("wb2api-panel-desktop.exe")
		}
	}

	if enable {
		_ = exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "WorkBuddy2APIPanel", "/t", "REG_SZ", "/d", `"`+desktopExe+`"`, "/f").Run()
	} else {
		_ = exec.Command("reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "WorkBuddy2APIPanel", "/f").Run()
	}
}

func isWindowsAutoStartActive() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	cmd := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "WorkBuddy2APIPanel")
	return cmd.Run() == nil
}

func (p *Panel) getDesktopSettings(w http.ResponseWriter, r *http.Request) {
	s := loadDesktopSettings()
	if runtime.GOOS == "windows" {
		s.AutoStart = isWindowsAutoStartActive()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"auto_start":      s.AutoStart,
		"start_minimized": s.StartMinimized,
		"is_windows":      runtime.GOOS == "windows",
	})
}

func (p *Panel) saveDesktopSettings(w http.ResponseWriter, r *http.Request) {
	var req DesktopSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	setWindowsAutoStart(req.AutoStart)
	_ = saveDesktopSettingsToFile(req)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"auto_start":      req.AutoStart,
		"start_minimized": req.StartMinimized,
	})
}
