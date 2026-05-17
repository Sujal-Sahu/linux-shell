package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// Confirm asks the user for yes/no confirmation.
func Confirm(prompt string) bool {
	yellow := color.New(color.FgYellow, color.Bold)
	yellow.Printf("  ⚠  %s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	resp, _ := reader.ReadString('\n')
	resp = strings.TrimSpace(strings.ToLower(resp))
	return resp == "y" || resp == "yes"
}

// RunCommand runs a shell command and returns its combined output.
func RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// RunCommandStdin runs a command with stdin piped through shell.
func RunShell(cmdStr string) (string, error) {
	cmd := exec.Command("sh", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// SectionHeader prints a styled section header.
func SectionHeader(title string) {
	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Println()
	cyan.Printf("  ┌─ %s ", title)
	fmt.Println(strings.Repeat("─", max(0, 50-len(title))))
}

// SectionEnd prints section footer.
func SectionEnd() {
	color.New(color.FgHiBlack).Println("  └" + strings.Repeat("─", 52))
}

// PrintKeyVal prints a key-value pair.
func PrintKeyVal(key, val string) {
	bold := color.New(color.FgWhite, color.Bold)
	bold.Printf("    %-22s", key+":")
	fmt.Println(val)
}

// PrintInfo prints an informational line.
func PrintInfo(format string, args ...interface{}) {
	color.New(color.FgHiWhite).Printf("    "+format+"\n", args...)
}

// PrintWarn prints a warning line.
func PrintWarn(format string, args ...interface{}) {
	color.New(color.FgYellow).Printf("  ⚠  "+format+"\n", args...)
}

// PrintGood prints a success/ok line.
func PrintGood(format string, args ...interface{}) {
	color.New(color.FgGreen).Printf("  ✓  "+format+"\n", args...)
}

// PrintBad prints an error/critical line.
func PrintBad(format string, args ...interface{}) {
	color.New(color.FgRed, color.Bold).Printf("  ✗  "+format+"\n", args...)
}

// PrintRow prints a table row.
func PrintRow(format string, args ...interface{}) {
	color.New(color.FgHiBlack).Printf("    "+format+"\n", args...)
}

// IsRoot checks if the current process is running as root.
func IsRoot() bool {
	return os.Getuid() == 0
}

// ReadLines reads all lines from a file.
func ReadLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// ParseInt safely parses an int.
func ParseInt(s string) int {
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}

// ParseFloat safely parses a float.
func ParseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// KillProcess sends SIGKILL to a PID.
func KillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// TruncStr truncates a string to max length.
func TruncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
