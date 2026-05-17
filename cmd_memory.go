package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// ─── OOM Killer ───────────────────────────────────────────────────────────────

type OomCandidate struct {
	PID        int
	Name       string
	OomScore   int
	OomAdj     int
	VmRSS      int // KB
	VmSwap     int // KB
}

func OomKiller(args []string) error {
	SectionHeader("OOM-KILLER — Out of Memory Analyzer")
	PrintInfo("Analyzing OOM scores and memory pressure...")
	fmt.Println()

	// System memory overview
	memInfo := getMemInfo()
	PrintKeyVal("Total RAM", fmt.Sprintf("%s MB", memInfo["MemTotal"]))
	PrintKeyVal("Available", fmt.Sprintf("%s MB", memInfo["MemAvailable"]))
	PrintKeyVal("Swap Total", fmt.Sprintf("%s MB", memInfo["SwapTotal"]))
	PrintKeyVal("Swap Free", fmt.Sprintf("%s MB", memInfo["SwapFree"]))
	fmt.Println()

	// Memory pressure level
	totalKB := ParseInt(memInfo["MemTotalKB"])
	availKB := ParseInt(memInfo["MemAvailableKB"])
	if totalKB > 0 {
		usedPct := float64(totalKB-availKB) / float64(totalKB) * 100
		if usedPct > 90 {
			PrintBad("Memory pressure CRITICAL: %.1f%% used!", usedPct)
		} else if usedPct > 75 {
			PrintWarn("Memory pressure HIGH: %.1f%% used", usedPct)
		} else {
			PrintGood("Memory pressure OK: %.1f%% used", usedPct)
		}
	}

	fmt.Println()

	candidates, err := getOomCandidates()
	if err != nil {
		return fmt.Errorf("failed to collect OOM info: %v", err)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].OomScore > candidates[j].OomScore
	})

	header := color.New(color.FgYellow, color.Bold)
	header.Printf("    %-8s %-22s %10s %10s %10s %10s\n",
		"PID", "NAME", "OOM_SCORE", "OOM_ADJ", "RSS(MB)", "SWAP(MB)")
	fmt.Println("    " + strings.Repeat("─", 76))

	limit := 15
	if len(candidates) < limit {
		limit = len(candidates)
	}

	for _, c := range candidates[:limit] {
		rssMB := float64(c.VmRSS) / 1024.0
		swapMB := float64(c.VmSwap) / 1024.0

		lineColor := color.New(color.FgHiWhite)
		if c.OomScore > 500 {
			lineColor = color.New(color.FgRed, color.Bold)
		} else if c.OomScore > 200 {
			lineColor = color.New(color.FgYellow)
		}

		lineColor.Printf("    %-8d %-22s %10d %10d %10.1f %10.1f\n",
			c.PID, TruncStr(c.Name, 22), c.OomScore, c.OomAdj, rssMB, swapMB)
	}

	fmt.Println()
	PrintInfo("Processes with highest OOM score are most likely to be killed by the kernel.")
	PrintInfo("Run 'oom-history' to see past OOM kill events.")

	// Offer to kill highest OOM score process
	if len(candidates) > 0 && candidates[0].OomScore > 100 {
		top := candidates[0]
		fmt.Println()
		prompt := fmt.Sprintf("Kill top OOM candidate '%s' (PID %d, score %d)?",
			top.Name, top.PID, top.OomScore)
		if Confirm(prompt) {
			if err := KillProcess(top.PID); err != nil {
				PrintBad("Failed to kill PID %d: %v", top.PID, err)
			} else {
				PrintGood("Sent SIGKILL to '%s' (PID %d). Memory should be released.", top.Name, top.PID)
			}
		}
	}

	SectionEnd()
	return nil
}

func getOomCandidates() ([]OomCandidate, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var candidates []OomCandidate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		scoreData, err := os.ReadFile(fmt.Sprintf("/proc/%d/oom_score", pid))
		if err != nil {
			continue
		}
		score := ParseInt(strings.TrimSpace(string(scoreData)))

		adjData, err := os.ReadFile(fmt.Sprintf("/proc/%d/oom_score_adj", pid))
		adj := 0
		if err == nil {
			adj = ParseInt(strings.TrimSpace(string(adjData)))
		}

		name := getProcessName(pid)

		status, _ := ReadLines(fmt.Sprintf("/proc/%d/status", pid))
		vmRSS := 0
		vmSwap := 0
		for _, line := range status {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			switch parts[0] {
			case "VmRSS:":
				vmRSS = ParseInt(parts[1])
			case "VmSwap:":
				vmSwap = ParseInt(parts[1])
			}
		}

		candidates = append(candidates, OomCandidate{
			PID:      pid,
			Name:     name,
			OomScore: score,
			OomAdj:   adj,
			VmRSS:    vmRSS,
			VmSwap:   vmSwap,
		})
	}
	return candidates, nil
}

func getMemInfo() map[string]string {
	result := map[string]string{}
	lines, err := ReadLines("/proc/meminfo")
	if err != nil {
		return result
	}

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		valKB := ParseInt(parts[1])

		result[key+"KB"] = parts[1]
		result[key] = strconv.Itoa(valKB / 1024)
	}
	return result
}

// ─── OOM History ──────────────────────────────────────────────────────────────

func OomHistory(args []string) error {
	SectionHeader("OOM-HISTORY — Out of Memory Kill Events")
	PrintInfo("Searching system logs for OOM kill events...")
	fmt.Println()

	// Try dmesg first
	output, err := RunShell("dmesg --time-format=reltime 2>/dev/null | grep -i 'oom\\|killed process\\|out of memory' | tail -50")
	if err != nil || strings.TrimSpace(output) == "" {
		// Try journalctl
		output, err = RunShell("journalctl -k --no-pager 2>/dev/null | grep -i 'oom\\|killed process\\|out of memory' | tail -50")
	}

	if err != nil || strings.TrimSpace(output) == "" {
		// Try /var/log/syslog or kern.log
		for _, logFile := range []string{"/var/log/kern.log", "/var/log/syslog", "/var/log/messages"} {
			if FileExists(logFile) {
				output, _ = RunShell(fmt.Sprintf("grep -i 'oom\\|killed process\\|out of memory' %s 2>/dev/null | tail -50", logFile))
				if strings.TrimSpace(output) != "" {
					break
				}
			}
		}
	}

	if strings.TrimSpace(output) == "" {
		PrintGood("No OOM kill events found in system logs. No memory panics detected!")
		SectionEnd()
		return nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	color.New(color.FgRed, color.Bold).Printf("  Found %d OOM-related log entries:\n\n", len(lines))

	for _, line := range lines {
		// Highlight key parts
		if strings.Contains(strings.ToLower(line), "killed process") {
			color.New(color.FgRed, color.Bold).Printf("    %s\n", TruncStr(line, 120))
		} else if strings.Contains(strings.ToLower(line), "out of memory") {
			color.New(color.FgYellow, color.Bold).Printf("    %s\n", TruncStr(line, 120))
		} else {
			color.New(color.FgHiBlack).Printf("    %s\n", TruncStr(line, 120))
		}
	}

	fmt.Println()
	PrintInfo("Tip: Use 'oom-killer' to see current OOM candidates and prevent future kills.")

	SectionEnd()
	return nil
}
