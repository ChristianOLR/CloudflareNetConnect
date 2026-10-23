//go:build darwin

package tray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	macSvcNotInstalled = 0
	macSvcStopped      = 1
	macSvcRunning      = 4
)

func launchAgentPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.github.cloudflarenetconnect.plist")
}

func isMacStartupEnabled() bool {
	_, err := os.Stat(launchAgentPlistPath())
	return err == nil
}

func setMacStartupEnabled(enable bool) {
	plistPath := launchAgentPlistPath()
	if !enable {
		_ = exec.Command("launchctl", "unload", plistPath).Run()
		_ = os.Remove(plistPath)
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = "/Applications/CloudflareNetConnect.app/Contents/MacOS/CloudflareNetConnect"
	}
	_ = os.MkdirAll(filepath.Dir(plistPath), 0755)

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.github.cloudflarenetconnect</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
`, exePath)

	_ = os.WriteFile(plistPath, []byte(plistContent), 0644)
}

func queryMacServiceStatus() int {
	daemonPlist := "/Library/LaunchDaemons/CloudflareNetConnect.plist"
	if _, err := os.Stat(daemonPlist); os.IsNotExist(err) {
		return macSvcNotInstalled
	}
	out, err := exec.Command("launchctl", "list").CombinedOutput()
	if err == nil && strings.Contains(string(out), "CloudflareNetConnect") {
		return macSvcRunning
	}
	return macSvcStopped
}

func runMacElevatedServiceCmd(action string) {
	exePath, err := os.Executable()
	if err != nil {
		exePath = "/Applications/CloudflareNetConnect.app/Contents/MacOS/CloudflareNetConnect"
	}
	var cmdStr string
	switch action {
	case "install_start":
		cmdStr = fmt.Sprintf(`"%s" -service install && "%s" -service start`, exePath, exePath)
	case "stop_uninstall":
		cmdStr = fmt.Sprintf(`"%s" -service stop ; "%s" -service uninstall`, exePath, exePath)
	default:
		cmdStr = fmt.Sprintf(`"%s" -service %s`, exePath, action)
	}
	appleScript := fmt.Sprintf(`do shell script %q with administrator privileges`, cmdStr)
	_ = exec.Command("osascript", "-e", appleScript).Run()
}
