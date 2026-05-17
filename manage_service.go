package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

// ── Service object ────────────────────────────────────────────────────────────
// Every field maps directly to a systemd unit directive.
// Optional fields are omitted when empty.

type ServiceObject struct {
	// [Unit]
	Name        string `json:"name"`
	Description string `json:"description"`
	After       string `json:"after"`        // e.g. "network.target"
	Requires    string `json:"requires"`     // hard dependency
	Wants       string `json:"wants"`        // soft dependency

	// [Service]
	Type           string `json:"type"`            // simple | forking | oneshot | notify | dbus
	ExecStart      string `json:"exec_start"`
	ExecStop       string `json:"exec_stop"`
	ExecReload     string `json:"exec_reload"`
	WorkingDir     string `json:"working_dir"`
	User           string `json:"user"`
	Group          string `json:"group"`
	Restart        string `json:"restart"`         // no | on-failure | always | on-abnormal
	RestartSec     string `json:"restart_sec"`     // seconds between restarts
	Environment    string `json:"environment"`     // KEY=VAL KEY2=VAL2 …
	EnvironmentFile string `json:"environment_file"` // path to env file
	CPUQuota       string `json:"cpu_quota"`       // e.g. "50%"
	MemoryMax      string `json:"memory_max"`      // e.g. "512M"
	TasksMax       string `json:"tasks_max"`
	LimitNOFILE    string `json:"limit_nofile"`    // open file descriptor limit
	StandardOutput string `json:"standard_output"` // journal | syslog | null | file:path
	StandardError  string `json:"standard_error"`
	TimeoutStartSec string `json:"timeout_start_sec"`
	TimeoutStopSec  string `json:"timeout_stop_sec"`

	// [Install]
	WantedBy string `json:"wanted_by"` // multi-user.target | graphical.target
}

// ── Router ────────────────────────────────────────────────────────────────────

func ServiceManager(args []string) error {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	rest := []string{}
	if len(args) > 1 {
		rest = args[1:]
	}

	switch sub {
	case "create":
		return svcCreate()
	case "list", "ls":
		return svcList()
	case "status":
		return svcStatus(rest)
	case "start":
		return svcControl("start", rest)
	case "stop":
		return svcControl("stop", rest)
	case "restart":
		return svcControl("restart", rest)
	case "reload":
		return svcControl("reload", rest)
	case "enable":
		return svcControl("enable", rest)
	case "disable":
		return svcControl("disable", rest)
	case "logs":
		return svcLogs(rest)
	case "edit":
		return svcEdit(rest)
	case "show":
		return svcShow(rest)
	case "delete", "rm":
		return svcDelete(rest)
	case "export":
		return svcExport(rest)
	case "import":
		return svcImport(rest)
	default:
		return svcHelp()
	}
}

// ── Help ──────────────────────────────────────────────────────────────────────

func svcHelp() error {
	SectionHeader("SERVICE — Systemd Service Manager")
	fmt.Println()

	color.New(color.FgYellow, color.Bold).Printf("    %-32s %s\n", "COMMAND", "DESCRIPTION")
	fmt.Println("    " + strings.Repeat("─", 76))

	rows := [][]string{
		{"service create", "Wizard: define every attribute of a new service, then deploy it"},
		{"service list", "All services: state, load, active status"},
		{"service status <name>", "Detailed status: uptime, PID, memory, last log lines"},
		{"service start   <name>", "Start a service"},
		{"service stop    <name>", "Stop a service"},
		{"service restart <name>", "Restart a service"},
		{"service reload  <name>", "Reload config without restarting (if ExecReload set)"},
		{"service enable  <name>", "Enable service to start at boot"},
		{"service disable <name>", "Prevent service from starting at boot"},
		{"service logs    <name> [n]", "Last N journal lines (default 50), coloured by severity"},
		{"service show    <name>", "Dump all unit properties (raw systemctl show)"},
		{"service edit    <name>", "Open unit file in $EDITOR (auto-backup first)"},
		{"service delete  <name>", "Stop → disable → remove unit file (backup kept)"},
		{"service export  <name>", "Write service definition to ./<name>.svc.json"},
		{"service import  <file>", "Re-create a service from an exported JSON file"},
	}
	for _, r := range rows {
		color.New(color.FgGreen, color.Bold).Printf("    %-32s", r[0])
		color.New(color.FgHiWhite).Printf("%s\n", r[1])
	}

	fmt.Println()
	PrintInfo("Tip: 'service' without a sub-command shows this help.")
	SectionEnd()
	return nil
}

// ── Create wizard ─────────────────────────────────────────────────────────────

func svcCreate() error {
	SectionHeader("SERVICE CREATE — New Systemd Service")
	WizardHeader("Fill in every attribute. Press Enter to accept the [default]. Ctrl+C to abort.")

	svc := ServiceObject{}

	// ── [Unit] ──────────────────────────────────────────────────────────────
	color.New(color.FgCyan, color.Bold).Println("  │  [Unit]")

	svc.Name = AskRequired("Service name (no .service suffix)")
	svc.Name = strings.TrimSuffix(svc.Name, ".service")

	unitPath := filepath.Join(unitDir, svc.Name+".service")
	if FileExists(unitPath) {
		WizardEnd()
		PrintWarn("'%s.service' already exists. Use 'service edit %s' to modify it.", svc.Name, svc.Name)
		SectionEnd()
		return nil
	}

	svc.Description    = Ask("Description", svc.Name+" managed service")
	svc.After          = Ask("After  (space-separated units)", "network.target")
	svc.Requires       = Ask("Requires  (hard deps, optional)", "")
	svc.Wants          = Ask("Wants     (soft deps, optional)", "")

	// ── [Service] ────────────────────────────────────────────────────────────
	fmt.Println("  │")
	color.New(color.FgCyan, color.Bold).Println("  │  [Service]")

	svc.Type = AskChoice("Type",
		[]string{"simple", "forking", "oneshot", "notify", "dbus"}, "simple")
	svc.ExecStart  = AskRequired("ExecStart  (full binary path + arguments)")
	svc.ExecStop   = Ask("ExecStop   (optional)", "")
	svc.ExecReload = Ask("ExecReload (optional, e.g. /bin/kill -HUP $MAINPID)", "")
	svc.WorkingDir = Ask("WorkingDirectory (optional)", "")
	svc.User       = Ask("User", "root")
	svc.Group      = Ask("Group", svc.User)
	svc.Restart    = AskChoice("Restart",
		[]string{"no", "on-failure", "always", "on-abnormal", "on-success"}, "on-failure")
	svc.RestartSec = Ask("RestartSec  (seconds)", "5")
	svc.Environment     = Ask("Environment  (KEY=VAL KEY2=VAL2, optional)", "")
	svc.EnvironmentFile = Ask("EnvironmentFile  (path, optional)", "")

	// ── Resources ────────────────────────────────────────────────────────────
	fmt.Println("  │")
	color.New(color.FgCyan, color.Bold).Println("  │  [Service] — Resource limits (leave blank = unlimited)")

	svc.CPUQuota    = Ask("CPUQuota    (e.g. 50%  = half a core)", "")
	svc.MemoryMax   = Ask("MemoryMax   (e.g. 512M, 2G)", "")
	svc.TasksMax    = Ask("TasksMax    (max threads+processes)", "")
	svc.LimitNOFILE = Ask("LimitNOFILE (open file descriptors)", "")

	// ── Logging ──────────────────────────────────────────────────────────────
	fmt.Println("  │")
	color.New(color.FgCyan, color.Bold).Println("  │  [Service] — Logging & Timeouts")

	svc.StandardOutput  = AskChoice("StandardOutput",
		[]string{"journal", "syslog", "null", "file:/var/log/" + svc.Name + ".log"}, "journal")
	svc.StandardError   = AskChoice("StandardError",
		[]string{"journal", "inherit", "null"}, "journal")
	svc.TimeoutStartSec = Ask("TimeoutStartSec  (seconds)", "90")
	svc.TimeoutStopSec  = Ask("TimeoutStopSec   (seconds)", "30")

	// ── [Install] ────────────────────────────────────────────────────────────
	fmt.Println("  │")
	color.New(color.FgCyan, color.Bold).Println("  │  [Install]")

	svc.WantedBy = AskChoice("WantedBy",
		[]string{"multi-user.target", "graphical.target", "network-online.target"}, "multi-user.target")

	WizardEnd()
	fmt.Println()

	// Preview
	unit := renderServiceUnit(svc)
	color.New(color.FgCyan, color.Bold).Printf("  ● Preview — %s\n", unitPath)
	printUnitBlock(unit)
	fmt.Println()

	if !Confirm(fmt.Sprintf("Write, enable and start '%s.service'?", svc.Name)) {
		PrintInfo("Aborted — nothing written.")
		SectionEnd()
		return nil
	}

	// Write unit file
	BackupFile(unitPath, "create", "service", svc.Name, "Created via service wizard")
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return fmt.Errorf("write unit file: %v", err)
	}
	PrintGood("Written: %s", unitPath)

	// Persist JSON so we can export/reconstruct later
	os.MkdirAll(lsSvcMeta, 0700)
	if data, err := json.MarshalIndent(svc, "", "  "); err == nil {
		os.WriteFile(filepath.Join(lsSvcMeta, svc.Name+".json"), data, 0600)
	}

	fmt.Println()
	printStep("Reload systemd daemon",   "systemctl daemon-reload")
	printStep("Enable at boot",          "systemctl enable "+svc.Name+".service")
	printStep("Start service",           "systemctl start "+svc.Name+".service")

	fmt.Println()
	color.New(color.FgCyan, color.Bold).Println("  ● Status after start")
	statusOut, _ := RunShell("systemctl status " + svc.Name + ".service --no-pager 2>&1 | head -14")
	for _, line := range strings.Split(statusOut, "\n") {
		colorStatusLine(line)
	}

	SectionEnd()
	return nil
}

// renderServiceUnit converts a ServiceObject to a unit file string.
func renderServiceUnit(s ServiceObject) string {
	var b strings.Builder

	line := func(l string)        { b.WriteString(l + "\n") }
	kv   := func(k, v string) {
		if v != "" {
			b.WriteString(k + "=" + v + "\n")
		}
	}

	line("[Unit]")
	kv("Description", s.Description)
	kv("After",       s.After)
	kv("Requires",    s.Requires)
	kv("Wants",       s.Wants)

	line("")
	line("[Service]")
	kv("Type",            s.Type)
	kv("ExecStart",       s.ExecStart)
	kv("ExecStop",        s.ExecStop)
	kv("ExecReload",      s.ExecReload)
	kv("WorkingDirectory", s.WorkingDir)
	kv("User",            s.User)
	if s.Group != "" && s.Group != s.User {
		kv("Group", s.Group)
	}
	kv("Restart",          s.Restart)
	kv("RestartSec",       s.RestartSec)
	kv("Environment",      s.Environment)
	kv("EnvironmentFile",  s.EnvironmentFile)
	kv("CPUQuota",         s.CPUQuota)
	kv("MemoryMax",        s.MemoryMax)
	kv("TasksMax",         s.TasksMax)
	kv("LimitNOFILE",      s.LimitNOFILE)
	kv("StandardOutput",   s.StandardOutput)
	kv("StandardError",    s.StandardError)
	kv("TimeoutStartSec",  s.TimeoutStartSec)
	kv("TimeoutStopSec",   s.TimeoutStopSec)

	line("")
	line("[Install]")
	kv("WantedBy", s.WantedBy)

	return b.String()
}

// ── List ──────────────────────────────────────────────────────────────────────

func svcList() error {
	SectionHeader("SERVICE LIST — All Systemd Services")
	fmt.Println()

	out, err := RunShell("systemctl list-units --type=service --no-pager --no-legend 2>/dev/null")
	if err != nil {
		return fmt.Errorf("systemctl unavailable: %v", err)
	}

	color.New(color.FgYellow, color.Bold).Printf(
		"    %-40s %-10s %-10s %s\n", "SERVICE", "LOAD", "ACTIVE", "DESCRIPTION")
	fmt.Println("    " + strings.Repeat("─", 86))

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		name, load, active := f[0], f[1], f[2]
		desc := ""
		if len(f) > 4 {
			desc = strings.Join(f[4:], " ")
		}
		c := color.New(color.FgHiWhite)
		switch active {
		case "active":
			c = color.New(color.FgGreen)
		case "failed":
			c = color.New(color.FgRed, color.Bold)
		case "inactive":
			c = color.New(color.FgHiBlack)
		}
		c.Printf("    %-40s %-10s %-10s %s\n",
			TruncStr(name, 40), load, active, TruncStr(desc, 44))
	}

	fmt.Println()
	PrintInfo("service status <name>  →  detailed view | service logs <name>  →  journal")
	SectionEnd()
	return nil
}

// ── Status ────────────────────────────────────────────────────────────────────

func svcStatus(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service status <name>")
	}
	name := strings.TrimSuffix(args[0], ".service")
	SectionHeader(fmt.Sprintf("SERVICE STATUS — %s.service", name))
	fmt.Println()

	out, _ := RunShell(fmt.Sprintf("systemctl status %s.service --no-pager 2>&1", name))
	for _, line := range strings.Split(out, "\n") {
		colorStatusLine(line)
	}

	SectionEnd()
	return nil
}

// ── Control (start / stop / restart / reload / enable / disable) ──────────────

func svcControl(action string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service %s <name>", action)
	}
	name := strings.TrimSuffix(args[0], ".service")

	out, err := RunShell(fmt.Sprintf("systemctl %s %s.service 2>&1", action, name))
	if err != nil {
		PrintBad("systemctl %s %s.service — %s", action, name, strings.TrimSpace(out))
		return nil
	}
	PrintGood("systemctl %s %s.service", action, name)

	// Show concise state afterwards
	state, _ := RunShell(fmt.Sprintf("systemctl is-active %s.service 2>/dev/null", name))
	PrintKeyVal("Current state", strings.TrimSpace(state))
	return nil
}

// ── Logs ──────────────────────────────────────────────────────────────────────

func svcLogs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service logs <name> [lines]")
	}
	name := strings.TrimSuffix(args[0], ".service")
	n    := "50"
	if len(args) > 1 {
		n = args[1]
	}

	SectionHeader(fmt.Sprintf("SERVICE LOGS — %s.service  (last %s lines)", name, n))
	fmt.Println()

	out, _ := RunShell(fmt.Sprintf("journalctl -u %s.service --no-pager -n %s 2>&1", name, n))
	for _, line := range strings.Split(out, "\n") {
		colorLogLine(line)
	}
	SectionEnd()
	return nil
}

// ── Show (raw properties) ─────────────────────────────────────────────────────

func svcShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service show <name>")
	}
	name := strings.TrimSuffix(args[0], ".service")
	SectionHeader(fmt.Sprintf("SERVICE SHOW — %s.service  (raw properties)", name))
	fmt.Println()

	// Curated list of interesting properties
	props := []string{
		"Id", "Description", "LoadState", "ActiveState", "SubState",
		"UnitFileState", "MainPID", "ExecStart", "ExecStop", "ExecReload",
		"User", "Group", "WorkingDirectory", "Restart", "RestartUSec",
		"Environment", "EnvironmentFiles",
		"CPUQuotaPerSecUSec", "MemoryMax", "TasksMax", "LimitNOFILE",
		"StandardOutput", "StandardError",
		"TimeoutStartUSec", "TimeoutStopUSec",
	}
	out, _ := RunShell(fmt.Sprintf(
		"systemctl show %s.service --property=%s --no-pager 2>/dev/null",
		name, strings.Join(props, ",")))

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := parts[0], parts[1]
		if v == "" || v == "0" || v == "[not set]" || v == "infinity" || v == "18446744073709551615" {
			color.New(color.FgHiBlack).Printf("    %-28s %s\n", k, v)
		} else {
			color.New(color.FgYellow, color.Bold).Printf("    %-28s", k)
			color.New(color.FgHiWhite).Printf("%s\n", v)
		}
	}
	SectionEnd()
	return nil
}

// ── Edit ──────────────────────────────────────────────────────────────────────

func svcEdit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service edit <name>")
	}
	name     := strings.TrimSuffix(args[0], ".service")
	unitPath := filepath.Join(unitDir, name+".service")

	if !FileExists(unitPath) {
		return fmt.Errorf("unit file not found: %s", unitPath)
	}

	BackupFile(unitPath, "edit", "service", name, "Pre-edit backup")
	PrintGood("Backup saved. Opening in editor...")

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}
	RunShell(editor + " " + unitPath)

	RunShell("systemctl daemon-reload 2>&1")
	PrintGood("Daemon reloaded. Run 'service restart %s' to apply changes.", name)
	return nil
}

// ── Delete ────────────────────────────────────────────────────────────────────

func svcDelete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service delete <name>")
	}
	name     := strings.TrimSuffix(args[0], ".service")
	unitPath := filepath.Join(unitDir, name+".service")

	SectionHeader(fmt.Sprintf("SERVICE DELETE — %s.service", name))
	fmt.Println()

	if !FileExists(unitPath) {
		return fmt.Errorf("unit file not found: %s", unitPath)
	}

	// Show current state so the user knows what they are deleting
	state, _ := RunShell(fmt.Sprintf("systemctl is-active %s.service 2>/dev/null", name))
	PrintKeyVal("Current state", strings.TrimSpace(state))
	PrintWarn("This will stop, disable and permanently remove %s.service", name)
	fmt.Println()

	if !Confirm(fmt.Sprintf("Delete '%s.service'? A backup of the unit file will be kept.", name)) {
		PrintInfo("Aborted — nothing changed.")
		SectionEnd()
		return nil
	}

	BackupFile(unitPath, "delete", "service", name, "Deleted via service manager")

	printStep("Stop service",    "systemctl stop "+name+".service")
	printStep("Disable service", "systemctl disable "+name+".service")

	if err := os.Remove(unitPath); err != nil {
		PrintBad("Remove unit file: %v", err)
	} else {
		PrintGood("Removed: %s", unitPath)
	}

	// Also remove saved JSON meta if present
	os.Remove(filepath.Join(lsSvcMeta, name+".json"))

	printStep("Reload daemon", "systemctl daemon-reload")

	fmt.Println()
	PrintInfo("Unit file backed up. Restore with: rollback list → rollback apply <id>")
	SectionEnd()
	return nil
}

// ── Export ────────────────────────────────────────────────────────────────────

func svcExport(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service export <name>")
	}
	name     := strings.TrimSuffix(args[0], ".service")
	unitPath := filepath.Join(unitDir, name+".service")

	if !FileExists(unitPath) {
		return fmt.Errorf("unit file not found: %s", unitPath)
	}

	content, err := os.ReadFile(unitPath)
	if err != nil {
		return fmt.Errorf("read unit file: %v", err)
	}

	// Try to also embed structured metadata if we have it
	payload := map[string]interface{}{
		"name":      name,
		"unit_file": string(content),
	}
	metaPath := filepath.Join(lsSvcMeta, name+".json")
	if FileExists(metaPath) {
		var svc ServiceObject
		if data, err := os.ReadFile(metaPath); err == nil {
			if json.Unmarshal(data, &svc) == nil {
				payload["service_object"] = svc
			}
		}
	}

	data, _ := json.MarshalIndent(payload, "", "  ")
	outFile  := name + ".svc.json"
	if err := os.WriteFile(outFile, data, 0644); err != nil {
		return fmt.Errorf("write export file: %v", err)
	}
	PrintGood("Exported to: %s", outFile)
	PrintInfo("Re-create on another host with: service import %s", outFile)
	return nil
}

// ── Import ────────────────────────────────────────────────────────────────────

func svcImport(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: service import <file.svc.json>")
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read file: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse JSON: %v", err)
	}

	var name string
	if err := json.Unmarshal(payload["name"], &name); err != nil || name == "" {
		return fmt.Errorf("JSON missing valid 'name' field")
	}
	var unitContent string
	if err := json.Unmarshal(payload["unit_file"], &unitContent); err != nil || unitContent == "" {
		return fmt.Errorf("JSON missing valid 'unit_file' field")
	}

	unitPath := filepath.Join(unitDir, name+".service")
	SectionHeader(fmt.Sprintf("SERVICE IMPORT — %s.service", name))
	fmt.Println()

	if FileExists(unitPath) {
		PrintWarn("'%s.service' already exists.", name)
		if !Confirm("Overwrite existing unit file?") {
			PrintInfo("Aborted.")
			SectionEnd()
			return nil
		}
		BackupFile(unitPath, "edit", "service", name, "Pre-import overwrite backup")
	} else {
		BackupFile(unitPath, "create", "service", name, "Imported via service import")
	}

	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("write unit file: %v", err)
	}
	PrintGood("Written: %s", unitPath)

	// Save structured object if present
	if raw, ok := payload["service_object"]; ok {
		os.MkdirAll(lsSvcMeta, 0700)
		os.WriteFile(filepath.Join(lsSvcMeta, name+".json"), raw, 0600)
	}

	printStep("Reload daemon", "systemctl daemon-reload")

	fmt.Println()
	PrintInfo("Service imported. Start it with: service start %s", name)
	SectionEnd()
	return nil
}
