package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
)

// ─── Disk Health ──────────────────────────────────────────────────────────────

func DiskHealth(args []string) error {
	SectionHeader("DISK-HEALTH — Disk Usage & I/O Analyzer")
	fmt.Println()

	// Filesystem usage
	color.New(color.FgCyan, color.Bold).Println("  ● Filesystem Usage")
	out, err := RunShell("df -hT 2>/dev/null")
	if err == nil {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for i, line := range lines {
			if i == 0 {
				color.New(color.FgYellow, color.Bold).Printf("    %s\n", line)
				continue
			}
			parts := strings.Fields(line)
			if len(parts) < 6 {
				color.New(color.FgHiBlack).Printf("    %s\n", line)
				continue
			}
			// Parse usage percentage
			usePct := strings.TrimSuffix(parts[5], "%")
			pct := ParseInt(usePct)
			lineColor := color.New(color.FgGreen)
			if pct >= 90 {
				lineColor = color.New(color.FgRed, color.Bold)
			} else if pct >= 75 {
				lineColor = color.New(color.FgYellow)
			}
			lineColor.Printf("    %s\n", line)
		}
	}

	fmt.Println()

	// Top 10 largest directories
	color.New(color.FgCyan, color.Bold).Println("  ● Top 10 Largest Directories (from /)")
	out, _ = RunShell("du -sh /* 2>/dev/null | sort -rh | head -10")
	if strings.TrimSpace(out) != "" {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			color.New(color.FgHiWhite).Printf("    %s\n", line)
		}
	}

	fmt.Println()

	// I/O stats
	color.New(color.FgCyan, color.Bold).Println("  ● Disk I/O Statistics")
	out, _ = RunShell("iostat -dx 1 1 2>/dev/null | grep -v '^$' | tail -20")
	if strings.TrimSpace(out) != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for i, line := range lines {
			if i == 0 || strings.HasPrefix(line, "Linux") || strings.HasPrefix(line, "avg-cpu") {
				color.New(color.FgYellow, color.Bold).Printf("    %s\n", line)
			} else {
				color.New(color.FgHiWhite).Printf("    %s\n", line)
			}
		}
	} else {
		// Fallback: read from /proc/diskstats
		PrintInfo("iostat not available. Reading /proc/diskstats...")
		lines, _ := ReadLines("/proc/diskstats")
		color.New(color.FgYellow, color.Bold).Printf("    %-10s %12s %12s %12s %12s\n",
			"DEVICE", "READS", "READ_KB", "WRITES", "WRITE_KB")
		fmt.Println("    " + strings.Repeat("─", 62))
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) < 14 {
				continue
			}
			dev := parts[2]
			// Skip loop and ram devices
			if strings.HasPrefix(dev, "loop") || strings.HasPrefix(dev, "ram") {
				continue
			}
			reads := parts[3]
			readKB := ParseInt(parts[5]) / 2 // 512b sectors to KB
			writes := parts[7]
			writeKB := ParseInt(parts[9]) / 2
			color.New(color.FgHiWhite).Printf("    %-10s %12s %12d %12s %12d\n",
				dev, reads, readKB, writes, writeKB)
		}
	}

	fmt.Println()

	// Check for any disk near full
	out, _ = RunShell("df -h 2>/dev/null")
	lines := strings.Split(out, "\n")
	hasWarning := false
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}
		usePct := strings.TrimSuffix(parts[4], "%")
		pct := ParseInt(usePct)
		if pct >= 90 {
			PrintBad("CRITICAL: %s is %.0f%% full (%s)", parts[len(parts)-1], float64(pct), parts[1])
			hasWarning = true
		} else if pct >= 80 {
			PrintWarn("WARNING: %s is %d%% full", parts[len(parts)-1], pct)
			hasWarning = true
		}
	}
	if !hasWarning {
		PrintGood("All filesystems have adequate free space.")
	}

	SectionEnd()
	return nil
}

// ─── Inode Crisis ─────────────────────────────────────────────────────────────

func InodeCrisis(args []string) error {
	SectionHeader("INODE-CRISIS — Inode Exhaustion Detector")
	PrintInfo("Checking inode usage across mounted filesystems...")
	fmt.Println()

	out, err := RunShell("df -i 2>/dev/null")
	if err != nil {
		return fmt.Errorf("cannot run df -i: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	hasCrisis := false

	header := color.New(color.FgYellow, color.Bold)
	header.Printf("    %-30s %10s %10s %10s %8s\n",
		"FILESYSTEM", "INODES", "USED", "FREE", "USE%")
	fmt.Println("    " + strings.Repeat("─", 72))

	type InodeStat struct {
		FS    string
		Total int
		Used  int
		Free  int
		Pct   int
		Mount string
	}
	var stats []InodeStat

	for i, line := range lines {
		if i == 0 {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 6 {
			continue
		}
		fs := parts[0]
		total := ParseInt(parts[1])
		used := ParseInt(parts[2])
		free := ParseInt(parts[3])
		pctStr := strings.TrimSuffix(parts[4], "%")
		pct := ParseInt(pctStr)
		mount := parts[5]

		stats = append(stats, InodeStat{FS: fs, Total: total, Used: used, Free: free, Pct: pct, Mount: mount})
	}

	sort.Slice(stats, func(i, j int) bool { return stats[i].Pct > stats[j].Pct })

	for _, s := range stats {
		lineColor := color.New(color.FgGreen)
		marker := ""
		if s.Pct >= 95 {
			lineColor = color.New(color.FgRed, color.Bold)
			marker = " ← CRITICAL"
			hasCrisis = true
		} else if s.Pct >= 80 {
			lineColor = color.New(color.FgYellow)
			marker = " ← WARNING"
			hasCrisis = true
		}
		lineColor.Printf("    %-30s %10d %10d %10d %7d%%%s\n",
			TruncStr(s.FS, 30), s.Total, s.Used, s.Free, s.Pct, marker)
	}

	fmt.Println()

	if hasCrisis {
		PrintBad("Inode exhaustion detected! Creating new files will fail on affected mounts.")
		fmt.Println()
		PrintInfo("Remediation tips:")
		color.New(color.FgHiWhite).Println("    • Find directories with many small files: find / -xdev -printf '%h\n' | sort | uniq -c | sort -rn | head -20")
		color.New(color.FgHiWhite).Println("    • Check for many temp files: ls /tmp | wc -l")
		color.New(color.FgHiWhite).Println("    • Check for many log files: find /var/log -type f | wc -l")
	} else {
		PrintGood("Inode usage is within safe limits on all filesystems.")
	}

	SectionEnd()
	return nil
}

// ─── Fsck Report ──────────────────────────────────────────────────────────────

func FsckReport(args []string) error {
	SectionHeader("FSCK-REPORT — Filesystem Check Log Analyzer")
	PrintInfo("Looking for filesystem errors in system logs...")
	fmt.Println()

	found := false

	// Check dmesg for filesystem errors
	color.New(color.FgCyan, color.Bold).Println("  ● Kernel Filesystem Messages (dmesg)")
	fsErrors, _ := RunShell(`dmesg 2>/dev/null | grep -iE 'ext4|xfs|btrfs|fsck|filesystem error|I/O error|bad block|corrupt|journal' | tail -30`)
	if strings.TrimSpace(fsErrors) != "" {
		found = true
		for _, line := range strings.Split(strings.TrimSpace(fsErrors), "\n") {
			if strings.ContainsAny(strings.ToLower(line), "error,corrupt,bad,fail") {
				color.New(color.FgRed, color.Bold).Printf("    %s\n", TruncStr(line, 120))
			} else {
				color.New(color.FgHiBlack).Printf("    %s\n", TruncStr(line, 120))
			}
		}
	} else {
		PrintGood("No filesystem errors in dmesg.")
	}

	fmt.Println()

	// Check /var/log for fsck output
	color.New(color.FgCyan, color.Bold).Println("  ● Boot Filesystem Check Results")
	for _, path := range []string{"/var/log/fsck/checkfs", "/var/log/boot.log", "/var/log/messages"} {
		if !FileExists(path) {
			continue
		}
		out, _ := RunShell(fmt.Sprintf("grep -i 'fsck\\|e2fsck\\|xfs_repair\\|filesystem' %s 2>/dev/null | tail -20", path))
		if strings.TrimSpace(out) != "" {
			found = true
			color.New(color.FgHiBlack).Printf("    From %s:\n", path)
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				color.New(color.FgHiWhite).Printf("    %s\n", TruncStr(line, 110))
			}
			break
		}
	}

	fmt.Println()

	// Mount options and filesystem types
	color.New(color.FgCyan, color.Bold).Println("  ● Currently Mounted Filesystems")
	out, _ := RunShell("mount | grep -vE 'proc|sysfs|devtmpfs|cgroup|tmpfs|devpts|mqueue|hugetlbfs|debugfs|securityfs|fusectl' 2>/dev/null")
	if strings.TrimSpace(out) != "" {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			color.New(color.FgHiWhite).Printf("    %s\n", TruncStr(line, 110))
		}
	}

	fmt.Println()

	// Check smart data if available
	color.New(color.FgCyan, color.Bold).Println("  ● SMART Health Check (block devices)")
	blkDevs, _ := RunShell("lsblk -dno NAME,TYPE 2>/dev/null | grep disk | awk '{print $1}'")
	if strings.TrimSpace(blkDevs) != "" {
		for _, dev := range strings.Split(strings.TrimSpace(blkDevs), "\n") {
			dev = strings.TrimSpace(dev)
			if dev == "" {
				continue
			}
			smartOut, err := RunShell(fmt.Sprintf("smartctl -H /dev/%s 2>/dev/null | grep -i 'health\\|result'", dev))
			if err == nil && strings.TrimSpace(smartOut) != "" {
				if strings.Contains(strings.ToLower(smartOut), "passed") || strings.Contains(strings.ToLower(smartOut), "ok") {
					PrintGood("/dev/%s: %s", dev, strings.TrimSpace(smartOut))
				} else {
					PrintBad("/dev/%s: %s", dev, strings.TrimSpace(smartOut))
				}
			} else {
				color.New(color.FgHiBlack).Printf("    /dev/%s: smartctl not available or no SMART data\n", dev)
			}
		}
	}

	if !found {
		fmt.Println()
		PrintGood("No filesystem errors found in logs. Your storage looks healthy!")
	}

	SectionEnd()
	return nil
}
