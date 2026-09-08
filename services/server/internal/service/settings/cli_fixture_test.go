package settings

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMain also supplies a real, portable child executable for CLI contracts.
func TestMain(m *testing.M) {
	if mode := os.Getenv("JW_SETTINGS_CLI_FIXTURE"); mode != "" {
		os.Exit(runSettingsCLIFixture(mode, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func writeSettingsCLIFixture(t *testing.T, mode, argsPath string) string {
	t.Helper()
	t.Setenv("JW_SETTINGS_CLI_FIXTURE", mode)
	t.Setenv("JW_SETTINGS_CLI_ARGS", argsPath)
	if mode == "libtv-pending" {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("JW_SETTINGS_CLI_LOCK", listener.Addr().String())
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name := "fixture-cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), name)
	if err := os.Link(source, binPath); err != nil {
		content, readErr := os.ReadFile(source)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err := os.WriteFile(binPath, content, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return binPath
}

func runSettingsCLIFixture(mode string, args []string) int {
	command := strings.Join(args, " ")
	if argsPath := os.Getenv("JW_SETTINGS_CLI_ARGS"); argsPath != "" {
		file, err := os.OpenFile(argsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return 2
		}
		_, err = fmt.Fprintln(file, command)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return 2
		}
	}
	switch mode {
	case "logout":
		if command == "logout" {
			return 0
		}
	case "jimeng-existing":
		if command == "login --headless" {
			fmt.Println("已复用当前本地 OAuth 登录态。")
			return 0
		}
	case "jimeng-headless":
		switch command {
		case "login --headless":
			fmt.Println("verification_uri: https://example.test/device\nuser_code: ABCD-EFGH\ndevice_code: device-123")
			// The real CLI returns a device challenge before browser approval and
			// exits non-zero. The service must still hand that challenge to the UI.
			return 1
		case "login checklogin --device_code=device-123 --poll=30":
			fmt.Println("即梦本地登录态已可用")
			return 0
		}
	case "libtv-success":
		if command == "login web" {
			fmt.Println("Open https://libtv.example.test/login in your browser")
			time.Sleep(200 * time.Millisecond)
			return 0
		}
	case "libtv-pending":
		if command == "logout" {
			return 0
		}
		if command == "login web" {
			// The OS releases this exclusive lock when the prior CLI exits.
			// A replacement login must not overlap the previous child process.
			listener, err := net.Listen("tcp", os.Getenv("JW_SETTINGS_CLI_LOCK"))
			if err != nil {
				fmt.Fprintln(os.Stderr, "LibTV login is already running")
				return 23
			}
			defer listener.Close()
			fmt.Println("Open https://libtv.example.test/login in your browser")
			for {
				time.Sleep(time.Second)
			}
		}
	}
	fmt.Fprintln(os.Stderr, "unexpected fixture command:", mode, command)
	return 1
}
