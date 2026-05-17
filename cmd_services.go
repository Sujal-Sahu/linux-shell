package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// ─── Service Trace ────────────────────────────────────────────────────────────

func ServiceTrace(args []string) error {
	SectionHeader("SERVICE-TRACE — Systemd Service Inspector")
	fmt.Println()

	if len(args) > 0 {
		// Trace specific service
		svc := args[0]
		if !strings.HasSuffix(svc, ".service") {
			svc = svc + ".service"
		}
		return traceService(svc)
	}

	// No specific service — show overview of failed/degraded
	color.New(color.FgCyan, color.Bold).Println("  ● System State")
	state, _ := RunShell("systemctl is-system-running 2>/dev/null")
	state = strings.TrimSpace(state)
	if state == "running" {
		PrintGood("System state: %s", state)
	} else if state == "degraded" {
		PrintWarn("System state: %s (some units have failed)", state)
	} else {
		PrintBad("System state: %s", state)
	}

	fmt.Println()

	// Failed units
	color.New(color.FgCyan, color.Bold).Println("  ● Failed Units")
	out, _ := RunShell("systemctl --failed --no-legend 2>/dev/null")
	if strings.TrimSpace(out) == "" {
		PrintGood("No failed units.")
	} else {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, line := range lines {
			color.New(color.FgRed, color.Bold).Printf("    %s\n", line)
		}
		fmt.Println()
		PrintInfo("Run: service-trace <service-name> to inspect a specific service")
	}

	fmt.Println()

	// Recently started/stopped
	color.New(color.FgCyan, color.Bold).Println("  ● Recent Service Activity (last 10 events)")
	out, _ = RunShell("journalctl -u '*.service' --no-pager -n 20 --output=short 2>/dev/null | grep -E 'Started|Stopped|Failed|Starting|Stopping' | tail -10")
	if strings.TrimSpace(out) != "" {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.Contains(line, "Failed") || strings.Contains(line, "fail") {
				color.New(color.FgRed).Printf("    %s\n", TruncStr(line, 110))
			} else if strings.Contains(line, "Started") {
				color.New(color.FgGreen).Printf("    %s\n", TruncStr(line, 110))
			} else if strings.Contains(line, "Stopped") {
				color.New(color.FgYellow).Printf("    %s\n", TruncStr(line, 110))
			} else {
				color.New(color.FgHiBlack).Printf("    %s\n", TruncStr(line, 110))
			}
		}
	}

	fmt.Println()

	// Top resource-consuming services
	color.New(color.FgCyan, color.Bold).Println("  ● Services by Memory Usage")
	out, _ = RunShell("systemctl show '*.service' --property=MemoryCurrent,Id --value 2>/dev/null | paste - - | sort -t$'\\t' -k1 -rn | head -10")
	if strings.TrimSpace(out) == "" {
		// fallback
		out, _ = RunShell("systemd-cgtop -n 1 -d 1 2>/dev/null | head -15")
	}
	if strings.TrimSpace(out) != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for i, line := range lines {
			if i == 0 {
				color.New(color.FgYellow, color.Bold).Printf("    %s\n", line)
			} else {
				color.New(color.FgHiWhite).Printf("    %s\n", TruncStr(line, 100))
			}
		}
	}

	SectionEnd()
	return nil
}

func traceService(svc string) error {
	color.New(color.FgCyan, color.Bold).Printf("  ● Status: %s\n", svc)
	out, _ := RunShell(fmt.Sprintf("systemctl status %s --no-pager 2>/dev/null", svc))
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Active: active") {
			color.New(color.FgGreen).Printf("    %s\n", line)
		} else if strings.Contains(line, "Active: failed") || strings.Contains(line, "failed") {
			color.New(color.FgRed, color.Bold).Printf("    %s\n", line)
		} else if strings.Contains(line, "Active: inactive") {
			color.New(color.FgYellow).Printf("    %s\n", line)
		} else {
			color.New(color.FgHiWhite).Printf("    %s\n", TruncStr(line, 110))
		}
	}

	fmt.Println()
	color.New(color.FgCyan, color.Bold).Printf("  ● Last 20 log lines: %s\n", svc)
	logs, _ := RunShell(fmt.Sprintf("journalctl -u %s --no-pager -n 20 2>/dev/null", svc))
	if strings.TrimSpace(logs) != "" {
		for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
			if strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "failed") {
				color.New(color.FgRed).Printf("    %s\n", TruncStr(line, 110))
			} else if strings.Contains(strings.ToLower(line), "warn") {
				color.New(color.FgYellow).Printf("    %s\n", TruncStr(line, 110))
			} else {
				color.New(color.FgHiBlack).Printf("    %s\n", TruncStr(line, 110))
			}
		}
	}

	fmt.Println()
	color.New(color.FgCyan, color.Bold).Printf("  ● Dependencies: %s\n", svc)
	deps, _ := RunShell(fmt.Sprintf("systemctl list-dependencies %s --no-pager 2>/dev/null | head -20", svc))
	if strings.TrimSpace(deps) != "" {
		for _, line := range strings.Split(strings.TrimSpace(deps), "\n") {
			color.New(color.FgHiBlack).Printf("    %s\n", line)
		}
	}

	SectionEnd()
	return nil
}

// ─── Cron Debug ───────────────────────────────────────────────────────────────

func CronDebug(args []string) error {
	SectionHeader("CRON-DEBUG — Cron Job Inspector")
	fmt.Println()

	// System crontab
	color.New(color.FgCyan, color.Bold).Println("  ● System Crontab (/etc/crontab)")
	if FileExists("/etc/crontab") {
		lines, _ := ReadLines("/etc/crontab")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "#") {
				color.New(color.FgHiBlack).Printf("    %s\n", line)
			} else {
				color.New(color.FgHiWhite).Printf("    %s\n", line)
			}
		}
	} else {
		color.New(color.FgHiBlack).Println("    /etc/crontab not found")
	}

	fmt.Println()

	// /etc/cron.d
	color.New(color.FgCyan, color.Bold).Println("  ● /etc/cron.d/ Jobs")
	out, _ := RunShell("ls /etc/cron.d/ 2>/dev/null")
	if strings.TrimSpace(out) != "" {
		for _, f := range strings.Split(strings.TrimSpace(out), "\n") {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			color.New(color.FgGreen).Printf("    📄 /etc/cron.d/%s\n", f)
			content, _ := ReadLines("/etc/cron.d/" + f)
			for _, line := range content {
				if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				color.New(color.FgHiBlack).Printf("       %s\n", TruncStr(line, 100))
			}
		}
	} else {
		color.New(color.FgHiBlack).Println("    No files in /etc/cron.d/")
	}

	fmt.Println()

	// User crontabs
	color.New(color.FgCyan, color.Bold).Println("  ● User Crontabs (/var/spool/cron)")
	for _, spoolDir := range []string{"/var/spool/cron/crontabs", "/var/spool/cron"} {
		if !FileExists(spoolDir) {
			continue
		}
		out, _ := RunShell(fmt.Sprintf("ls %s 2>/dev/null", spoolDir))
		if strings.TrimSpace(out) == "" {
			color.New(color.FgHiBlack).Println("    No user crontabs found")
			continue
		}
		for _, user := range strings.Split(strings.TrimSpace(out), "\n") {
			user = strings.TrimSpace(user)
			if user == "" {
				continue
			}
			color.New(color.FgGreen).Printf("    👤 User: %s\n", user)
			content, _ := ReadLines(spoolDir + "/" + user)
			for _, line := range content {
				if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
					continue
				}
				color.New(color.FgHiBlack).Printf("       %s\n", TruncStr(line, 100))
			}
		}
		break
	}

	fmt.Println()

	// Recent cron executions from syslog
	color.New(color.FgCyan, color.Bold).Println("  ● Recent Cron Executions (last 20)")
	cronLogs := ""
	for _, logf := range []string{"/var/log/syslog", "/var/log/cron", "/var/log/messages"} {
		if FileExists(logf) {
			cronLogs, _ = RunShell(fmt.Sprintf("grep -i CRON %s 2>/dev/null | tail -20", logf))
			if strings.TrimSpace(cronLogs) != "" {
				break
			}
		}
	}
	if cronLogs == "" {
		cronLogs, _ = RunShell("journalctl -u cron.service --no-pager -n 20 2>/dev/null")
	}
	if strings.TrimSpace(cronLogs) != "" {
		for _, line := range strings.Split(strings.TrimSpace(cronLogs), "\n") {
			if strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "fail") {
				color.New(color.FgRed).Printf("    %s\n", TruncStr(line, 110))
			} else {
				color.New(color.FgHiBlack).Printf("    %s\n", TruncStr(line, 110))
			}
		}
	} else {
		color.New(color.FgHiBlack).Println("    No cron log entries found")
	}

	SectionEnd()
	return nil
}
