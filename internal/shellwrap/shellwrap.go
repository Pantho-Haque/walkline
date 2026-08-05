package shellwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const wrapperBash = `
# walkline git wrapper - paste this into your .bashrc or .zshrc
git() {
    if [ "$1" = "push" ]; then
        # Capture range BEFORE push (remote ref updates after push completes)
        local range
        range=$(command git rev-parse --abbrev-ref --symbolic-full-name "@{u}" >/dev/null 2>&1 \
            && echo "@{u}..HEAD" || echo "HEAD")
        command git "$@"
        local status=$?
        if [ $status -eq 0 ]; then
            walkline mark-pushed "$range" 2>/dev/null
        fi
        return $status
    else
        command git "$@"
    fi
}
`

const wrapperZsh = `
# walkline git wrapper - paste this into your .bashrc or .zshrc
git() {
    if [[ "$1" = "push" ]]; then
        # Capture range BEFORE push (remote ref updates after push completes)
        local wl_range
        if command git rev-parse --abbrev-ref --symbolic-full-name "@{u}" >/dev/null 2>&1; then
            wl_range="@{u}..HEAD"
        else
            wl_range="HEAD"
        fi
        command git "$@"
        local wl_status=$?
        if [[ $wl_status -eq 0 ]]; then
            walkline mark-pushed "$wl_range" 2>/dev/null
        fi
        return $wl_status
    else
        command git "$@"
    fi
}
`

func Generate() (string, error) {
	shell, err := detectShell()
	if err != nil {
		return "", err
	}
	if shell == "zsh" {
		return wrapperZsh, nil
	}
	return wrapperBash, nil
}

func DetectShellRC() []string {
	var rcs []string
	home, _ := os.UserHomeDir()

	if isShell("bash") {
		if f := filepath.Join(home, ".bashrc"); fileExists(f) {
			rcs = append(rcs, f)
		}
		if f := filepath.Join(home, ".bash_profile"); fileExists(f) {
			rcs = append(rcs, f)
		}
	}
	if isShell("zsh") {
		if f := filepath.Join(home, ".zshrc"); fileExists(f) {
			rcs = append(rcs, f)
		}
		if f := filepath.Join(home, ".zprofile"); fileExists(f) {
			rcs = append(rcs, f)
		}
	}
	return rcs
}

func Install(wrapper string) error {
	rcs := DetectShellRC()
	if len(rcs) == 0 {
		return fmt.Errorf("no shell rc files found")
	}

	for _, rc := range rcs {
		content, err := os.ReadFile(rc)
		if err != nil {
			continue
		}
		if strings.Contains(string(content), "walkline git wrapper") {
			fmt.Printf("Wrapper already in %s, skipping\n", rc)
			continue
		}

		f, err := os.OpenFile(rc, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.WriteString(wrapper); err != nil {
			return err
		}
		fmt.Printf("Installed wrapper to %s\n", rc)
	}
	return nil
}

func detectShell() (string, error) {
	shell := os.Getenv("SHELL")
	if strings.Contains(shell, "zsh") {
		return "zsh", nil
	}
	return "bash", nil
}

func isShell(name string) bool {
	shell := filepath.Base(os.Getenv("SHELL"))
	return shell == name
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Remove() {
	rcs := DetectShellRC()
	for _, rc := range rcs {
		content, err := os.ReadFile(rc)
		if err != nil {
			continue
		}
		if !strings.Contains(string(content), "walkline git wrapper") {
			continue
		}
		// Remove the wrapper block
		lines := strings.Split(string(content), "\n")
		var newLines []string
		inWrapper := false
		for _, line := range lines {
			if strings.Contains(line, "# walkline git wrapper") {
				inWrapper = true
				continue
			}
			if inWrapper {
				if strings.HasPrefix(line, "}") && !strings.Contains(line, "#") {
					inWrapper = false
					continue
				}
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
			}
			newLines = append(newLines, line)
		}
		os.WriteFile(rc, []byte(strings.Join(newLines, "\n")), 0644)
	}
}
