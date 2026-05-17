package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
)

// ── Storage paths ─────────────────────────────────────────────────────────────

const (
	lsDir      = "/root/.linuxshell"           // root state dir
	lsBackups  = "/root/.linuxshell/backups"    // file backups
	lsHistory  = "/root/.linuxshell/history.json"
	lsSvcMeta  = "/root/.linuxshell/services"   // service JSON exports
	lsJobMeta  = "/root/.linuxshell/jobs"       // job JSON metadata
	unitDir    = "/etc/systemd/system"          // systemd unit files
)

// ── BackupEntry ───────────────────────────────────────────────────────────────

type BackupEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Action     string    `json:"action"`      // create | edit | delete
	ObjectType string    `json:"object_type"` // service | job
	ObjectName string    `json:"object_name"`
	OrigFile   string    `json:"orig_file"`
	BackupPath string    `json:"backup_path"` // empty if file didn't exist yet
	Notes      string    `json:"notes"`
}

var changeHistory []BackupEntry

// ── Bootstrap ─────────────────────────────────────────────────────────────────

func init() {
	for _, d := range []string{lsDir, lsBackups, lsSvcMeta, lsJobMeta} {
		os.MkdirAll(d, 0700)
	}
	loadChangeHistory()
}

func loadChangeHistory() {
	data, err := os.ReadFile(lsHistory)
	if err != nil {
		return
	}
	json.Unmarshal(data, &changeHistory)
}

func saveChangeHistory() {
	data, _ := json.MarshalIndent(changeHistory, "", "  ")
	os.WriteFile(lsHistory, data, 0600)
}

// ── BackupFile ────────────────────────────────────────────────────────────────
// Call this BEFORE modifying any system file.
// If the file already exists it is copied to lsBackups.
// Returns the backup entry ID (for display).

func BackupFile(origPath, action, objType, objName, notes string) string {
	id := fmt.Sprintf("%s-%s-%s",
		objType, objName, time.Now().Format("20060102-150405"))

	entry := BackupEntry{
		ID:         id,
		Timestamp:  time.Now(),
		Action:     action,
		ObjectType: objType,
		ObjectName: objName,
		OrigFile:   origPath,
		Notes:      notes,
	}

	if FileExists(origPath) {
		dst := filepath.Join(lsBackups, id+filepath.Ext(origPath))
		if data, err := os.ReadFile(origPath); err == nil {
			os.WriteFile(dst, data, 0600)
			entry.BackupPath = dst
		}
	}

	changeHistory = append(changeHistory, entry)
	saveChangeHistory()
	return id
}

// ── RollbackManager ───────────────────────────────────────────────────────────

func RollbackManager(args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "", "list":
		return rollbackList()
	case "apply":
		if len(args) < 2 {
			return fmt.Errorf("usage: rollback apply <id>")
		}
		return rollbackApply(args[1])
	case "clear":
		return rollbackClear()
	default:
		SectionHeader("ROLLBACK — Usage")
		PrintInfo("rollback list           list all tracked changes with IDs")
		PrintInfo("rollback apply <id>     restore a backed-up file to its original path")
		PrintInfo("rollback clear          wipe history (backup files are kept on disk)")
		SectionEnd()
		return nil
	}
}

func rollbackList() error {
	SectionHeader("ROLLBACK — Change History")
	fmt.Println()

	if len(changeHistory) == 0 {
		PrintInfo("No changes recorded yet.")
		PrintInfo("Every service/job create, edit, or delete is logged here automatically.")
		SectionEnd()
		return nil
	}

	// Newest first
	sorted := make([]BackupEntry, len(changeHistory))
	copy(sorted, changeHistory)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})

	hdr := color.New(color.FgYellow, color.Bold)
	hdr.Printf("    %-36s %-8s %-10s %-18s %s\n",
		"ID", "ACTION", "TYPE", "NAME", "TIMESTAMP")
	fmt.Println("    " + strings.Repeat("─", 90))

	for _, e := range sorted {
		c := color.New(color.FgGreen)
		switch e.Action {
		case "delete":
			c = color.New(color.FgRed)
		case "edit":
			c = color.New(color.FgYellow)
		}

		restorable := e.BackupPath != "" && FileExists(e.BackupPath)
		c.Printf("    %-36s %-8s %-10s %-18s %s",
			TruncStr(e.ID, 36), e.Action, e.ObjectType,
			TruncStr(e.ObjectName, 18),
			e.Timestamp.Format("2006-01-02 15:04:05"))

		if restorable {
			color.New(color.FgGreen).Printf("  ✓ restorable\n")
		} else {
			color.New(color.FgHiBlack).Printf("  (no backup)\n")
		}

		if e.Notes != "" {
			color.New(color.FgHiBlack).Printf("    %s  └─ %s\n", strings.Repeat(" ", 36), e.Notes)
		}
	}

	fmt.Println()
	PrintInfo("To restore a file: rollback apply <id>")
	SectionEnd()
	return nil
}

func rollbackApply(id string) error {
	SectionHeader("ROLLBACK — Apply")
	fmt.Println()

	var entry *BackupEntry
	for i := range changeHistory {
		if changeHistory[i].ID == id {
			entry = &changeHistory[i]
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("no entry found with ID '%s' — run 'rollback list' to see IDs", id)
	}

	PrintKeyVal("ID", entry.ID)
	PrintKeyVal("Action was", entry.Action)
	PrintKeyVal("Object", entry.ObjectType+"/"+entry.ObjectName)
	PrintKeyVal("Restoring to", entry.OrigFile)
	PrintKeyVal("From backup", entry.BackupPath)
	fmt.Println()

	if entry.BackupPath == "" || !FileExists(entry.BackupPath) {
		return fmt.Errorf("no backup file on disk for this entry (action '%s' had no pre-existing file to back up)", entry.Action)
	}

	if !Confirm(fmt.Sprintf("Overwrite '%s' with backup?", entry.OrigFile)) {
		PrintInfo("Cancelled.")
		SectionEnd()
		return nil
	}

	data, err := os.ReadFile(entry.BackupPath)
	if err != nil {
		return fmt.Errorf("read backup: %v", err)
	}
	if err := os.WriteFile(entry.OrigFile, data, 0644); err != nil {
		return fmt.Errorf("write original path: %v", err)
	}
	PrintGood("Restored '%s'.", entry.OrigFile)

	if entry.ObjectType == "service" || entry.ObjectType == "job" {
		RunShell("systemctl daemon-reload 2>/dev/null")
		PrintGood("systemd daemon reloaded.")
	}
	SectionEnd()
	return nil
}

func rollbackClear() error {
	if !Confirm("Clear all rollback history? Backup files on disk are kept.") {
		return nil
	}
	changeHistory = []BackupEntry{}
	saveChangeHistory()
	PrintGood("History cleared.")
	return nil
}

// ── Interactive prompt helpers ─────────────────────────────────────────────────

// Ask shows a prompt with an optional default value.
// The user presses Enter to accept the default.
func Ask(label, defaultVal string) string {
	c := color.New(color.FgCyan)
	if defaultVal != "" {
		c.Printf("    %-26s [%s]: ", label, defaultVal)
	} else {
		c.Printf("    %-26s: ", label)
	}
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// AskRequired keeps asking until the user provides a non-empty value.
func AskRequired(label string) string {
	for {
		val := Ask(label, "")
		if val != "" {
			return val
		}
		color.New(color.FgRed).Printf("    ✗  %s is required.\n", label)
	}
}

// AskChoice shows choices inline; returns the user's pick or the default.
func AskChoice(label string, choices []string, defaultVal string) string {
	c := color.New(color.FgCyan)
	c.Printf("    %-26s [%s] (%s): ", label, defaultVal, strings.Join(choices, " | "))
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	for _, ch := range choices {
		if strings.EqualFold(line, ch) {
			return ch
		}
	}
	PrintWarn("'%s' is not a valid choice — using default '%s'.", line, defaultVal)
	return defaultVal
}

// WizardHeader / WizardEnd bracket a wizard prompt block visually.
func WizardHeader(subtitle string) {
	fmt.Println()
	color.New(color.FgCyan, color.Bold).Printf("  ┌─ %s\n", subtitle)
	color.New(color.FgHiBlack).Println("  │  Enter to accept [default]  •  Ctrl+C to cancel")
	color.New(color.FgCyan, color.Bold).Println("  │")
}

func WizardEnd() {
	fmt.Println()
	color.New(color.FgCyan, color.Bold).Println("  └" + strings.Repeat("─", 54))
}

// printStep prints a "→ Doing thing... OK/FAILED" line.
func printStep(label, cmd string) {
	color.New(color.FgYellow).Printf("  → %-40s", label+"...")
	out, err := RunShell(cmd + " 2>&1")
	if err != nil {
		color.New(color.FgRed, color.Bold).Printf("FAILED\n")
		color.New(color.FgRed).Printf("    %s\n", strings.TrimSpace(out))
	} else {
		color.New(color.FgGreen, color.Bold).Printf("OK\n")
	}
}

// printUnitBlock pretty-prints a unit file with section headers highlighted.
func printUnitBlock(unit string) {
	color.New(color.FgHiBlack).Println("    " + strings.Repeat("─", 56))
	for _, line := range strings.Split(unit, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			color.New(color.FgCyan, color.Bold).Printf("    %s\n", line)
		} else {
			color.New(color.FgHiWhite).Printf("    %s\n", line)
		}
	}
	color.New(color.FgHiBlack).Println("    " + strings.Repeat("─", 56))
}

// colorLogLine prints a journal log line with severity-based colour.
func colorLogLine(line string) {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "fatal"):
		color.New(color.FgRed).Printf("    %s\n", line)
	case strings.Contains(lower, "warn"):
		color.New(color.FgYellow).Printf("    %s\n", line)
	case strings.Contains(lower, "start") || strings.Contains(lower, "running") || strings.Contains(lower, "success"):
		color.New(color.FgGreen).Printf("    %s\n", line)
	default:
		color.New(color.FgHiBlack).Printf("    %s\n", line)
	}
}

// colorStatusLine prints a systemctl status line with state-based colour.
func colorStatusLine(line string) {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "active (running)"):
		color.New(color.FgGreen, color.Bold).Printf("    %s\n", line)
	case strings.Contains(lower, "failed") || strings.Contains(lower, "error"):
		color.New(color.FgRed).Printf("    %s\n", line)
	case strings.Contains(lower, "inactive") || strings.Contains(lower, "activating"):
		color.New(color.FgYellow).Printf("    %s\n", line)
	default:
		color.New(color.FgHiWhite).Printf("    %s\n", line)
	}
}
