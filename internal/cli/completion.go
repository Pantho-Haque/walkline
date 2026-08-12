package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func installCompletion() {
	home, _ := os.UserHomeDir()

	sh := filepath.Base(os.Getenv("SHELL"))
	if sh == "" {
		fmt.Println("Shell completion: could not detect shell")
		return
	}

	switch sh {
	case "zsh":
		installZshCompletion(home)
	case "bash":
		installBashCompletion(home)
	case "fish":
		installFishCompletion(home)
	case "powershell", "pwsh":
		installPowerShellCompletion(home)
	default:
		fmt.Printf("Shell completion: unsupported shell '%s'. Supported: zsh, bash, fish, powershell\n", sh)
		fmt.Println("Run 'walkline completion <shell>' to generate the script manually.")
	}
}

func installZshCompletion(home string) {
	compDir := filepath.Join(home, ".zsh", "completions")
	os.MkdirAll(compDir, 0755)
	c := exec.Command("walkline", "completion", "zsh")
	out, err := c.Output()
	if err != nil {
		fmt.Println("Shell completion: failed to generate zsh completion")
		return
	}
	err = os.WriteFile(filepath.Join(compDir, "_walkline"), out, 0644)
	if err != nil {
		fmt.Println("Shell completion: failed to install zsh completion")
		return
	}

	zshrc := filepath.Join(home, ".zshrc")
	fpathLine := `fpath=($HOME/.zsh/completions $fpath)`
	content, _ := os.ReadFile(zshrc)
	if !strings.Contains(string(content), fpathLine) {
		f, _ := os.OpenFile(zshrc, os.O_WRONLY|os.O_APPEND, 0644)
		f.WriteString("\n" + fpathLine + "\n")
		f.Close()
	}
	fmt.Println("Zsh completion installed to ~/.zsh/completions/_walkline")
	fmt.Println("Restart shell or run: source ~/.zshrc")
}

func installBashCompletion(home string) {
	compFile := filepath.Join(home, ".bash_completion.d", "walkline")
	os.MkdirAll(filepath.Dir(compFile), 0755)
	c := exec.Command("walkline", "completion", "bash")
	out, err := c.Output()
	if err != nil {
		fmt.Println("Shell completion: failed to generate bash completion")
		return
	}
	err = os.WriteFile(compFile, out, 0644)
	if err != nil {
		fmt.Println("Shell completion: failed to install bash completion")
		return
	}
	fmt.Println("Bash completion installed to ~/.bash_completion.d/walkline")
}

func installFishCompletion(home string) {
	compDir := filepath.Join(home, ".config", "fish", "completions")
	os.MkdirAll(compDir, 0755)
	c := exec.Command("walkline", "completion", "fish")
	out, err := c.Output()
	if err != nil {
		fmt.Println("Shell completion: failed to generate fish completion")
		return
	}
	err = os.WriteFile(filepath.Join(compDir, "walkline.fish"), out, 0644)
	if err != nil {
		fmt.Println("Shell completion: failed to install fish completion")
		return
	}
	fmt.Println("Fish completion installed to ~/.config/fish/completions/walkline.fish")
}

func installPowerShellCompletion(home string) {
	compDir := filepath.Join(home, "Documents", "PowerShell", "Completions")
	os.MkdirAll(compDir, 0755)
	c := exec.Command("walkline", "completion", "powershell")
	out, err := c.Output()
	if err != nil {
		fmt.Println("Shell completion: failed to generate powershell completion")
		return
	}
	err = os.WriteFile(filepath.Join(compDir, "walkline.ps1"), out, 0644)
	if err != nil {
		fmt.Println("Shell completion: failed to install powershell completion")
		return
	}
	fmt.Println("PowerShell completion installed to ~/Documents/PowerShell/Completions/walkline.ps1")
}
