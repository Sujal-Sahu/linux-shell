package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// ─── Boot Timeline ────────────────────────────────────────────────────────────

func BootTimeline(args []string) error {
	SectionHeader("BOOT-TIMELINE — System Boot Analysis")
	fmt.Println()

	// Overall boot time
	color.New(color.FgCyan, color.Bold).Println("  ● Boot Time Summary")
	out, err := RunShell("systemd-analyze 2>/dev/null")
	if err == nil && strings.TrimSpace(out) != "" {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.Contains(line, "Startup finished") || strings.Contains(line, "graphical") {
				color.New(color.FgGreen, color.Bold).Printf("    %s\n", line)
			} else {
				color.New(color.FgHiWhite).Printf("    %s\n", line)
			}
		}
	} else {
		PrintWarn("systemd-analyze not available")
	}

	fmt.Println()

	// Boot blame — top slowest services
	color.New(color.FgCyan, color.Bold).Println("  ● Top 15 Slowest Services (boot blame)")
	out, _ = RunShell("systemd-analyze blame --no-pager 2>/dev/null | head -15")
	if strings.TrimSpace(out) != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for i, line := range lines {
			lineColor := color.New(color.FgHiWhite)
			if i == 0 {
				lineColor = color.New(color.FgRed, color.Bold)
			} else if i < 3 {
				lineColor = color.New(color.FgYellow)
			}
			lineColor.Printf("    %s\n", line)
		}
	} else {
		PrintWarn("systemd-analyze blame not available")
	}

	fmt.Println()

	// Critical chain
	color.New(color.FgCyan, color.Bold).Println("  ● Critical Boot Chain")
	out, _ = RunShell("systemd-analyze critical-chain --no-pager 2>/dev/null | head -25")
	if strings.TrimSpace(out) != "" {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if strings.Contains(line, "@") {
				color.New(color.FgCyan).Printf("    %s\n", line)
			} else {
				color.New(color.FgHiBlack).Printf("    %s\n", TruncStr(line, 100))
			}
		}
	}

	fmt.Println()

	// Last boot time
	color.New(color.FgCyan, color.Bold).Println("  ● System Uptime & Last Boot")
	uptime, _ := RunShell("uptime 2>/dev/null")
	who, _ := RunShell("who -b 2>/dev/null")
	lastBoot, _ := RunShell("last reboot 2>/dev/null | head -5")

	if strings.TrimSpace(uptime) != "" {
		PrintKeyVal("Uptime", strings.TrimSpace(uptime))
	}
	if strings.TrimSpace(who) != "" {
		PrintKeyVal("System boot", strings.TrimSpace(who))
	}

	if strings.TrimSpace(lastBoot) != "" {
		fmt.Println()
		color.New(color.FgCyan, color.Bold).Println("  ● Recent Reboots")
		for _, line := range strings.Split(strings.TrimSpace(lastBoot), "\n") {
			color.New(color.FgHiWhite).Printf("    %s\n", line)
		}
	}

	fmt.Println()

	// Kernel version
	color.New(color.FgCyan, color.Bold).Println("  ● Kernel Info")
	uname, _ := RunShell("uname -a 2>/dev/null")
	if strings.TrimSpace(uname) != "" {
		color.New(color.FgHiWhite).Printf("    %s\n", strings.TrimSpace(uname))
	}

	SectionEnd()
	return nil
}

// ─── Recent Logins ────────────────────────────────────────────────────────────

func RecentLogins(args []string) error {
	SectionHeader("RECENT-LOGINS — Login Activity Inspector")
	fmt.Println()

	// Successful logins
	color.New(color.FgCyan, color.Bold).Println("  ● Recent Successful Logins (last 20)")
	out, err := RunShell("last -n 20 --time-format=iso 2>/dev/null || last -n 20 2>/dev/null")
	if err == nil && strings.TrimSpace(out) != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) == 0 {
				continue
			}
			user := parts[0]
			switch {
			case user == "reboot":
				color.New(color.FgHiBlack).Printf("    %s\n", TruncStr(line, 100))
			case user == "root":
				color.New(color.FgRed, color.Bold).Printf("    %s\n", TruncStr(line, 100))
			default:
				color.New(color.FgGreen).Printf("    %s\n", TruncStr(line, 100))
			}
		}
	} else {
		PrintWarn("Could not read login history")
	}

	fmt.Println()

	// Failed login attempts
	color.New(color.FgCyan, color.Bold).Println("  ● Failed Login Attempts")

	// Try lastb first
	failedOut, err := RunShell("lastb -n 20 2>/dev/null")
	if err != nil || strings.TrimSpace(failedOut) == "" {
		// Try auth.log / secure
		for _, logf := range []string{"/var/log/auth.log", "/var/log/secure"} {
			if !FileExists(logf) {
				continue
			}
			failedOut, _ = RunShell(fmt.Sprintf(
				"grep -i 'failed password\\|authentication failure\\|invalid user' %s 2>/dev/null | tail -20", logf))
			if strings.TrimSpace(failedOut) != "" {
				break
			}
		}
	}

	if strings.TrimSpace(failedOut) == "" {
		// Try journalctl
		failedOut, _ = RunShell("journalctl _SYSTEMD_UNIT=sshd.service --no-pager -n 30 2>/dev/null | grep -i 'failed\\|invalid\\|refused'")
	}

	if strings.TrimSpace(failedOut) != "" {
		for _, line := range strings.Split(strings.TrimSpace(failedOut), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			color.New(color.FgRed).Printf("    %s\n", TruncStr(line, 110))
		}
	} else {
		PrintGood("No failed login attempts found (or log access restricted).")
	}

	fmt.Println()

	// Currently logged in users
	color.New(color.FgCyan, color.Bold).Println("  ● Currently Logged In Users")
	who, _ := RunShell("who 2>/dev/null")
	w, _ := RunShell("w --no-header 2>/dev/null")

	if strings.TrimSpace(who) != "" {
		header := color.New(color.FgYellow, color.Bold)
		header.Printf("    %-15s %-10s %-20s %s\n", "USER", "TTY", "FROM", "LOGIN@")
		fmt.Println("    " + strings.Repeat("─", 55))
		for _, line := range strings.Split(strings.TrimSpace(who), "\n") {
			color.New(color.FgHiWhite).Printf("    %s\n", line)
		}
	} else {
		PrintInfo("No interactive users currently logged in.")
	}

	if strings.TrimSpace(w) != "" {
		fmt.Println()
		color.New(color.FgCyan, color.Bold).Println("  ● Active Session Activity")
		for _, line := range strings.Split(strings.TrimSpace(w), "\n") {
			color.New(color.FgHiWhite).Printf("    %s\n", TruncStr(line, 110))
		}
	}

	fmt.Println()

	// Top failed login sources
	color.New(color.FgCyan, color.Bold).Println("  ● Top Brute Force Source IPs")
	for _, logf := range []string{"/var/log/auth.log", "/var/log/secure"} {
		if !FileExists(logf) {
			continue
		}
		ipCounts, _ := RunShell(fmt.Sprintf(
			`grep -i 'failed password' %s 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | sort | uniq -c | sort -rn | head -10`, logf))
		if strings.TrimSpace(ipCounts) != "" {
			header := color.New(color.FgYellow, color.Bold)
			header.Printf("    %8s  %s\n", "ATTEMPTS", "SOURCE IP")
			fmt.Println("    " + strings.Repeat("─", 30))
			for _, line := range strings.Split(strings.TrimSpace(ipCounts), "\n") {
				color.New(color.FgRed).Printf("    %s\n", line)
			}
		}
		break
	}

	SectionEnd()
	return nil
}
