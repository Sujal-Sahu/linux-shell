package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

// ── Job object ────────────────────────────────────────────────────────────────
// A "job" is a pair of units: <name>.timer + <name>-job.service.
// The timer drives the schedule; the service runs the command.

type JobObject struct {
	// Identity
	Name        string `json:"name"`
	Description string `json:"description"`

	// What to run
	Command    string `json:"command"`     // full path + args
	User       string `json:"user"`        // run as this user
	WorkingDir string `json:"working_dir"` // optional WorkingDirectory

	// When to run (timer)
	Schedule   string `json:"schedule"`     // OnCalendar expression
	OnBootSec  string `json:"on_boot_sec"`  // also run N after boot (e.g. "5min")
	Persistent bool   `json:"persistent"`   // run missed executions when system comes back

	// Output
	LogToFile bool   `json:"log_to_file"`  // true → append to /var/log/<name>-job.log
	LogFile   string `json:"log_file"`     // explicit path (auto-set when LogToFile=true)

	// Environment
	Environment     string `json:"environment"`      // KEY=VAL KEY2=VAL2 …
	EnvironmentFile string `json:"environment_file"` // path to env file

	// Resource limits on the job's service unit
	CPUQuota  string `json:"cpu_quota"`
	MemoryMax string `json:"memory_max"`
}

// ── Router ────────────────────────────────────────────────────────────────────

func JobManager(args []string) error {
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
		return jobCreate()
	case "list", "ls":
		return jobList()
	case "status":
		return jobStatus(rest)
	case "run":
		return jobRun(rest)
	case "logs":
		return jobLogs(rest)
	case "enable":
		return jobToggle("enable", rest)
	case "disable":
		return jobToggle("disable", rest)
	case "delete", "rm":
		return jobDelete(rest)
	case "validate":
		return jobValidate(rest)
	case "schedules":
		return jobScheduleRef()
	default:
		return jobHelp()
	}
}

// ── Help ──────────────────────────────────────────────────────────────────────

func jobHelp() error {
	SectionHeader("JOB — Scheduled Job Manager  (systemd timers)")
	fmt.Println()

	color.New(color.FgYellow, color.Bold).Printf("    %-32s %s\n", "COMMAND", "DESCRIPTION")
	fmt.Println("    " + strings.Repeat("─", 76))

	rows := [][]string{
		{"job create", "Wizard: define every attribute, preview units, then deploy"},
		{"job list", "All timers: last run, next run, associated service"},
		{"job status   <name>", "Timer state + service state + last execution result"},
		{"job run      <name>", "Trigger the job right now (bypasses schedule)"},
		{"job logs     <name> [n]", "Last N journal lines from the job service (default 50)"},
		{"job enable   <name>", "Enable and start the timer"},
		{"job disable  <name>", "Disable and stop the timer (definition kept)"},
		{"job delete   <name>", "Remove timer + service units (backup kept)"},
		{"job validate <name>", "Check schedule syntax, verify command exists"},
		{"job schedules", "OnCalendar expression reference with examples"},
	}
	for _, r := range rows {
		color.New(color.FgGreen, color.Bold).Printf("    %-32s", r[0])
		color.New(color.FgHiWhite).Printf("%s\n", r[1])
	}

	fmt.Println()
	PrintInfo("Each job creates two systemd units:")
	PrintInfo("  <name>.timer         — drives the schedule")
	PrintInfo("  <name>-job.service   — runs the command")
	fmt.Println()
	PrintInfo("'job schedules' shows OnCalendar expression examples.")
	SectionEnd()
	return nil
}

// ── Schedule reference ────────────────────────────────────────────────────────

func jobScheduleRef() error {
	SectionHeader("JOB SCHEDULES — OnCalendar Expression Reference")
	fmt.Println()

	color.New(color.FgYellow, color.Bold).Printf("    %-32s %s\n", "EXPRESSION", "MEANING")
	fmt.Println("    " + strings.Repeat("─", 72))

	examples := [][]string{
		{"minutely",                  "Every minute"},
		{"hourly",                    "Every hour at :00"},
		{"daily",                     "Every day at midnight (00:00:00)"},
		{"weekly",                    "Every Monday at midnight"},
		{"monthly",                   "1st of every month at midnight"},
		{"quarterly",                 "Jan 1 / Apr 1 / Jul 1 / Oct 1"},
		{"*:0/5",                     "Every 5 minutes"},
		{"*:0/15",                    "Every 15 minutes"},
		{"*:0/30",                    "Every 30 minutes"},
		{"*-*-* 02:00:00",            "Every day at 2:00 AM"},
		{"*-*-* 09:30:00",            "Every day at 9:30 AM"},
		{"Mon..Fri *-*-* 08:00:00",   "Weekdays at 8:00 AM"},
		{"Sat,Sun *-*-* 10:00:00",    "Weekends at 10:00 AM"},
		{"Mon *-*-* 03:00:00",        "Every Monday at 3:00 AM"},
		{"*-*-1 00:00:00",            "1st day of every month"},
		{"*-01,07-01 00:00:00",       "Jan 1 and Jul 1 (twice a year)"},
		{"*-*-* 00:00:00",            "Same as 'daily'"},
	}
	for _, e := range examples {
		color.New(color.FgGreen, color.Bold).Printf("    %-32s", e[0])
		color.New(color.FgHiWhite).Printf("%s\n", e[1])
	}

	fmt.Println()
	PrintInfo("Validate any expression: systemd-analyze calendar '<expression>'")
	PrintInfo("Full syntax docs: man systemd.time")
	SectionEnd()
	return nil
}

// ── Create wizard ─────────────────────────────────────────────────────────────

func jobCreate() error {
	SectionHeader("JOB CREATE — New Scheduled Job")
	WizardHeader("Fill in every attribute. Press Enter to accept the [default]. Ctrl+C to abort.")

	job := JobObject{Persistent: true}

	// ── Identity ─────────────────────────────────────────────────────────────
	color.New(color.FgCyan, color.Bold).Println("  │  Identity")

	job.Name = AskRequired("Job name  (no .timer suffix)")
	job.Name = strings.TrimSuffix(strings.TrimSuffix(job.Name, ".timer"), "-job")

	timerPath := filepath.Join(unitDir, job.Name+".timer")
	if FileExists(timerPath) {
		WizardEnd()
		PrintWarn("'%s.timer' already exists.", job.Name)
		SectionEnd()
		return nil
	}

	job.Description = Ask("Description", job.Name+" scheduled job")

	// ── What to run ───────────────────────────────────────────────────────────
	fmt.Println("  │")
	color.New(color.FgCyan, color.Bold).Println("  │  What to run")

	job.Command    = AskRequired("Command  (full path + arguments)")
	job.User       = Ask("Run as user", "root")
	job.WorkingDir = Ask("WorkingDirectory (optional)", "")
	job.Environment     = Ask("Environment  (KEY=VAL KEY2=VAL2, optional)", "")
	job.EnvironmentFile = Ask("EnvironmentFile  (path, optional)", "")

	// ── Schedule ──────────────────────────────────────────────────────────────
	fmt.Println("  │")
	color.New(color.FgCyan, color.Bold).Println("  │  Schedule  (type 'job schedules' for expression reference)")

	job.Schedule  = Ask("OnCalendar expression", "daily")
	job.OnBootSec = Ask("OnBootSec  (run N after boot, e.g. 5min — blank to skip)", "")
	persistent   := AskChoice("Persistent  (run missed executions on next boot)",
		[]string{"yes", "no"}, "yes")
	job.Persistent = persistent == "yes"

	// ── Logging ───────────────────────────────────────────────────────────────
	fmt.Println("  │")
	color.New(color.FgCyan, color.Bold).Println("  │  Output & Logging")

	logDest := AskChoice("Log output to", []string{"journal", "file"}, "journal")
	job.LogToFile = logDest == "file"
	if job.LogToFile {
		job.LogFile = Ask("Log file path", "/var/log/"+job.Name+"-job.log")
	}

	// ── Resources ─────────────────────────────────────────────────────────────
	fmt.Println("  │")
	color.New(color.FgCyan, color.Bold).Println("  │  Resource limits (leave blank = unlimited)")

	job.CPUQuota  = Ask("CPUQuota  (e.g. 50%)", "")
	job.MemoryMax = Ask("MemoryMax (e.g. 256M)", "")

	WizardEnd()
	fmt.Println()

	// Validate schedule before writing anything
	color.New(color.FgYellow).Printf("  → Validating schedule '%s'...", job.Schedule)
	calOut, calErr := RunShell("systemd-analyze calendar '" + job.Schedule + "' 2>&1")
	if calErr == nil && strings.Contains(calOut, "Next elapse") {
		color.New(color.FgGreen, color.Bold).Printf("  valid ✓\n")
		for _, line := range strings.Split(calOut, "\n") {
			if strings.TrimSpace(line) != "" {
				color.New(color.FgHiBlack).Printf("    %s\n", strings.TrimSpace(line))
			}
		}
	} else {
		color.New(color.FgYellow, color.Bold).Printf("  could not verify (systemd-analyze unavailable — continuing)\n")
	}
	fmt.Println()

	// Preview both units
	timerUnit := renderTimerUnit(job)
	svcUnit   := renderJobServiceUnit(job)

	color.New(color.FgCyan, color.Bold).Printf("  ● Preview — %s\n", timerPath)
	printUnitBlock(timerUnit)
	fmt.Println()
	svcPath := filepath.Join(unitDir, job.Name+"-job.service")
	color.New(color.FgCyan, color.Bold).Printf("  ● Preview — %s\n", svcPath)
	printUnitBlock(svcUnit)
	fmt.Println()

	if !Confirm(fmt.Sprintf("Write and enable job '%s'?", job.Name)) {
		PrintInfo("Aborted — nothing written.")
		SectionEnd()
		return nil
	}

	// Write units
	BackupFile(timerPath, "create", "job", job.Name, "Created via job wizard")
	if err := os.WriteFile(timerPath, []byte(timerUnit), 0644); err != nil {
		return fmt.Errorf("write timer unit: %v", err)
	}
	PrintGood("Written: %s", timerPath)

	if err := os.WriteFile(svcPath, []byte(svcUnit), 0644); err != nil {
		return fmt.Errorf("write service unit: %v", err)
	}
	PrintGood("Written: %s", svcPath)

	// Save JSON metadata
	os.MkdirAll(lsJobMeta, 0700)
	if data, err := json.MarshalIndent(job, "", "  "); err == nil {
		os.WriteFile(filepath.Join(lsJobMeta, job.Name+".json"), data, 0600)
	}

	fmt.Println()
	printStep("Reload systemd daemon",   "systemctl daemon-reload")
	printStep("Enable timer",            "systemctl enable "+job.Name+".timer")
	printStep("Start timer",             "systemctl start "+job.Name+".timer")

	fmt.Println()
	// Show next scheduled run
	nextOut, _ := RunShell("systemctl status " + job.Name + ".timer --no-pager 2>&1 | grep -E 'Trigger:|Active:|Triggers:'")
	for _, line := range strings.Split(strings.TrimSpace(nextOut), "\n") {
		if strings.TrimSpace(line) != "" {
			color.New(color.FgGreen).Printf("    %s\n", strings.TrimSpace(line))
		}
	}

	SectionEnd()
	return nil
}

// renderTimerUnit produces the .timer unit file content.
func renderTimerUnit(j JobObject) string {
	var b strings.Builder
	line := func(s string) { b.WriteString(s + "\n") }
	kv   := func(k, v string) {
		if v != "" {
			b.WriteString(k + "=" + v + "\n")
		}
	}

	line("[Unit]")
	kv("Description", j.Description+" (timer)")
	line("")
	line("[Timer]")
	kv("OnCalendar", j.Schedule)
	if j.OnBootSec != "" {
		kv("OnBootSec", j.OnBootSec)
	}
	if j.Persistent {
		line("Persistent=true")
	}
	line("Unit=" + j.Name + "-job.service")
	line("")
	line("[Install]")
	line("WantedBy=timers.target")
	return b.String()
}

// renderJobServiceUnit produces the -job.service unit file content.
func renderJobServiceUnit(j JobObject) string {
	var b strings.Builder
	line := func(s string) { b.WriteString(s + "\n") }
	kv   := func(k, v string) {
		if v != "" {
			b.WriteString(k + "=" + v + "\n")
		}
	}

	line("[Unit]")
	kv("Description", j.Description)
	line("")
	line("[Service]")
	line("Type=oneshot")
	kv("ExecStart",       j.Command)
	kv("User",            j.User)
	kv("WorkingDirectory", j.WorkingDir)
	kv("Environment",      j.Environment)
	kv("EnvironmentFile",  j.EnvironmentFile)
	kv("CPUQuota",         j.CPUQuota)
	kv("MemoryMax",        j.MemoryMax)

	if j.LogToFile && j.LogFile != "" {
		line("StandardOutput=append:" + j.LogFile)
		line("StandardError=append:" + j.LogFile)
	} else {
		line("StandardOutput=journal")
		line("StandardError=journal")
	}
	return b.String()
}

// ── List ──────────────────────────────────────────────────────────────────────

func jobList() error {
	SectionHeader("JOB LIST — Systemd Timers")
	fmt.Println()

	out, err := RunShell("systemctl list-timers --no-pager --no-legend 2>/dev/null")
	if err != nil || strings.TrimSpace(out) == "" {
		PrintInfo("No active timers found on this system.")
		PrintInfo("Create one with: job create")
		SectionEnd()
		return nil
	}

	// list-timers output: NEXT LEFT LAST PASSED UNIT ACTIVATES
	color.New(color.FgYellow, color.Bold).Printf(
		"    %-28s %-28s %-24s %s\n", "NEXT RUN", "LAST RUN", "TIMER", "SERVICE")
	fmt.Println("    " + strings.Repeat("─", 104))

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Colour timers we created (they end in .timer and have a -job.service)
		if strings.Contains(line, "-job.service") {
			color.New(color.FgGreen).Printf("    %s\n", TruncStr(line, 108))
		} else {
			color.New(color.FgHiBlack).Printf("    %s\n", TruncStr(line, 108))
		}
	}

	fmt.Println()
	PrintInfo("job status <name>  →  detail view | job run <name>  →  trigger now")
	SectionEnd()
	return nil
}

// ── Status ────────────────────────────────────────────────────────────────────

func jobStatus(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: job status <name>")
	}
	name := cleanJobName(args[0])
	SectionHeader(fmt.Sprintf("JOB STATUS — %s", name))
	fmt.Println()

	for _, unit := range []string{name + ".timer", name + "-job.service"} {
		color.New(color.FgCyan, color.Bold).Printf("  ● %s\n", unit)
		out, _ := RunShell("systemctl status " + unit + " --no-pager 2>&1 | head -16")
		for _, line := range strings.Split(out, "\n") {
			colorStatusLine(line)
		}
		fmt.Println()
	}

	// Show stored metadata
	metaPath := filepath.Join(lsJobMeta, name+".json")
	if FileExists(metaPath) {
		var job JobObject
		if data, _ := os.ReadFile(metaPath); len(data) > 0 && json.Unmarshal(data, &job) == nil {
			color.New(color.FgCyan, color.Bold).Println("  ● Job Definition")
			PrintKeyVal("Command",    job.Command)
			PrintKeyVal("Schedule",   job.Schedule)
			PrintKeyVal("User",       job.User)
			PrintKeyVal("Persistent", fmt.Sprintf("%v", job.Persistent))
			if job.LogToFile {
				PrintKeyVal("Log file", job.LogFile)
			}
		}
	}

	SectionEnd()
	return nil
}

// ── Run (immediate trigger) ───────────────────────────────────────────────────

func jobRun(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: job run <name>")
	}
	name    := cleanJobName(args[0])
	svcUnit := name + "-job.service"

	SectionHeader(fmt.Sprintf("JOB RUN — %s  (immediate trigger)", name))
	fmt.Println()

	PrintInfo("Starting %s right now (bypassing schedule)...", svcUnit)
	out, err := RunShell("systemctl start " + svcUnit + " 2>&1")
	if err != nil {
		PrintBad("Failed to trigger: %s", strings.TrimSpace(out))
		SectionEnd()
		return nil
	}
	PrintGood("Job triggered.")

	fmt.Println()
	color.New(color.FgCyan, color.Bold).Println("  ● Output (last 30 journal lines)")
	logs, _ := RunShell("journalctl -u " + svcUnit + " --no-pager -n 30 2>&1")
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		colorLogLine(line)
	}

	SectionEnd()
	return nil
}

// ── Logs ──────────────────────────────────────────────────────────────────────

func jobLogs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: job logs <name> [lines]")
	}
	name := cleanJobName(args[0])
	n    := "50"
	if len(args) > 1 {
		n = args[1]
	}

	SectionHeader(fmt.Sprintf("JOB LOGS — %s-job.service  (last %s lines)", name, n))
	fmt.Println()

	out, _ := RunShell(fmt.Sprintf("journalctl -u %s-job.service --no-pager -n %s 2>&1", name, n))
	for _, line := range strings.Split(out, "\n") {
		colorLogLine(line)
	}
	SectionEnd()
	return nil
}

// ── Enable / Disable ──────────────────────────────────────────────────────────

func jobToggle(action string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: job %s <name>", action)
	}
	name := cleanJobName(args[0])
	timer := name + ".timer"

	out, err := RunShell(fmt.Sprintf("systemctl %s %s 2>&1", action, timer))
	if err != nil {
		PrintBad("systemctl %s %s: %s", action, timer, strings.TrimSpace(out))
		return nil
	}
	PrintGood("systemctl %s %s", action, timer)

	// Start/stop the timer immediately too
	if action == "enable" {
		RunShell("systemctl start " + timer + " 2>/dev/null")
	} else {
		RunShell("systemctl stop " + timer + " 2>/dev/null")
	}

	state, _ := RunShell("systemctl is-active " + timer + " 2>/dev/null")
	PrintKeyVal("Timer state", strings.TrimSpace(state))
	return nil
}

// ── Delete ────────────────────────────────────────────────────────────────────

func jobDelete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: job delete <name>")
	}
	name      := cleanJobName(args[0])
	timerPath := filepath.Join(unitDir, name+".timer")
	svcPath   := filepath.Join(unitDir, name+"-job.service")

	SectionHeader(fmt.Sprintf("JOB DELETE — %s", name))
	fmt.Println()

	if !FileExists(timerPath) && !FileExists(svcPath) {
		return fmt.Errorf("no units found for job '%s'", name)
	}

	PrintWarn("This will remove %s.timer and %s-job.service", name, name)
	fmt.Println()

	if !Confirm(fmt.Sprintf("Delete job '%s'? Unit file backups will be kept.", name)) {
		PrintInfo("Aborted — nothing changed.")
		SectionEnd()
		return nil
	}

	BackupFile(timerPath, "delete", "job", name, "Deleted via job manager")

	printStep("Stop timer",    "systemctl stop "+name+".timer")
	printStep("Disable timer", "systemctl disable "+name+".timer")

	for _, path := range []string{timerPath, svcPath} {
		if FileExists(path) {
			if err := os.Remove(path); err != nil {
				PrintBad("Remove %s: %v", path, err)
			} else {
				PrintGood("Removed: %s", path)
			}
		}
	}

	// Remove metadata
	os.Remove(filepath.Join(lsJobMeta, name+".json"))

	printStep("Reload daemon", "systemctl daemon-reload")

	fmt.Println()
	PrintInfo("Restore with: rollback list → rollback apply <id>")
	SectionEnd()
	return nil
}

// ── Validate ──────────────────────────────────────────────────────────────────

func jobValidate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: job validate <name>")
	}
	name      := cleanJobName(args[0])
	timerPath := filepath.Join(unitDir, name+".timer")
	svcPath   := filepath.Join(unitDir, name+"-job.service")

	SectionHeader(fmt.Sprintf("JOB VALIDATE — %s", name))
	fmt.Println()

	// File existence
	if FileExists(timerPath) {
		PrintGood("Timer unit exists:   %s", timerPath)
	} else {
		PrintBad("Timer unit missing:  %s", timerPath)
	}
	if FileExists(svcPath) {
		PrintGood("Service unit exists: %s", svcPath)
	} else {
		PrintBad("Service unit missing: %s", svcPath)
	}
	fmt.Println()

	// Unit syntax
	if out, err := RunShell("systemd-analyze verify " + timerPath + " 2>&1"); err == nil && strings.TrimSpace(out) == "" {
		PrintGood("Timer unit syntax: valid")
	} else if err == nil {
		PrintWarn("Timer syntax warnings:\n    %s", strings.TrimSpace(out))
	} else {
		PrintWarn("systemd-analyze not available — skipping syntax check")
	}
	if out, err := RunShell("systemd-analyze verify " + svcPath + " 2>&1"); err == nil && strings.TrimSpace(out) == "" {
		PrintGood("Service unit syntax: valid")
	} else if err == nil {
		PrintWarn("Service syntax warnings:\n    %s", strings.TrimSpace(out))
	}

	// Metadata checks
	metaPath := filepath.Join(lsJobMeta, name+".json")
	if !FileExists(metaPath) {
		fmt.Println()
		PrintWarn("No metadata file — job was created outside the job manager or metadata was lost.")
		SectionEnd()
		return nil
	}

	var job JobObject
	data, _ := os.ReadFile(metaPath)
	if err := json.Unmarshal(data, &job); err != nil {
		PrintBad("Metadata file corrupt: %v", err)
		SectionEnd()
		return nil
	}

	// Schedule validation
	fmt.Println()
	PrintInfo("Schedule expression: %s", job.Schedule)
	calOut, calErr := RunShell("systemd-analyze calendar '" + job.Schedule + "' 2>&1")
	if calErr == nil && strings.Contains(calOut, "Next elapse") {
		PrintGood("Schedule is valid:")
		for _, line := range strings.Split(calOut, "\n") {
			if strings.TrimSpace(line) != "" {
				color.New(color.FgHiWhite).Printf("    %s\n", strings.TrimSpace(line))
			}
		}
	} else {
		PrintBad("Schedule validation failed: %s", strings.TrimSpace(calOut))
	}

	// Command check
	fmt.Println()
	binary := strings.Fields(job.Command)[0]
	_, err := RunShell("test -x " + binary + " 2>/dev/null || which " + binary + " 2>/dev/null")
	if err == nil {
		PrintGood("Command binary found: %s", binary)
	} else {
		PrintBad("Command binary not found or not executable: %s", binary)
	}

	// User check
	if job.User != "" && job.User != "root" {
		_, err := RunShell("id " + job.User + " 2>/dev/null")
		if err == nil {
			PrintGood("User exists: %s", job.User)
		} else {
			PrintBad("User not found: %s", job.User)
		}
	}

	SectionEnd()
	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// cleanJobName strips common suffixes so the user can pass any form.
func cleanJobName(s string) string {
	s = strings.TrimSuffix(s, ".timer")
	s = strings.TrimSuffix(s, "-job.service")
	s = strings.TrimSuffix(s, ".service")
	return s
}
