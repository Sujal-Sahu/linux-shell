package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// ─── Zombie Hunter ────────────────────────────────────────────────────────────

type ZombieProcess struct {
	PID    int
	PPID   int
	Name   string
	State  string
	Parent string
}

func ZombieHunter(args []string) error {
	SectionHeader("ZOMBIE HUNTER")
	PrintInfo("Scanning for zombie (defunct) processes...")
	fmt.Println()

	zombies, err := findZombies()
	if err != nil {
		return fmt.Errorf("could not read /proc: %v", err)
	}

	if len(zombies) == 0 {
		PrintGood("No zombie processes found. System is clean!")
		SectionEnd()
		return nil
	}

	red := color.New(color.FgRed, color.Bold)
	red.Printf("  Found %d zombie process(es):\n\n", len(zombies))

	header := color.New(color.FgYellow, color.Bold)
	header.Printf("    %-8s %-8s %-20s %-20s\n", "PID", "PPID", "NAME", "PARENT")
	fmt.Println("    " + strings.Repeat("─", 60))

	for _, z := range zombies {
		color.New(color.FgRed).Printf("    %-8d %-8d %-20s %-20s\n", z.PID, z.PPID, z.Name, z.Parent)
	}

	fmt.Println()
	PrintInfo("Note: Zombies are reaped by killing their parent process.")
	fmt.Println()

	for _, z := range zombies {
		prompt := fmt.Sprintf("Kill parent of zombie '%s' (PID %d, Parent PID %d)?", z.Name, z.PID, z.PPID)
		if Confirm(prompt) {
			if err := KillProcess(z.PPID); err != nil {
				PrintBad("Failed to kill parent PID %d: %v", z.PPID, err)
			} else {
				PrintGood("Sent SIGKILL to parent PID %d — zombie '%s' should be reaped.", z.PPID, z.Name)
			}
		} else {
			PrintInfo("Skipped zombie '%s' (PID %d).", z.Name, z.PID)
		}
	}

	SectionEnd()
	return nil
}

func findZombies() ([]ZombieProcess, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var zombies []ZombieProcess
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			continue
		}

		fields := parseProcStat(string(stat))
		if len(fields) < 4 {
			continue
		}

		state := fields[2]
		if state != "Z" {
			continue
		}

		name := strings.Trim(fields[1], "()")
		ppid := ParseInt(fields[3])
		parentName := getProcessName(ppid)

		zombies = append(zombies, ZombieProcess{
			PID:    pid,
			PPID:   ppid,
			Name:   name,
			State:  state,
			Parent: parentName,
		})
	}
	return zombies, nil
}

// parseProcStat handles the comm field which may contain spaces
func parseProcStat(stat string) []string {
	start := strings.Index(stat, "(")
	end := strings.LastIndex(stat, ")")
	if start < 0 || end < 0 {
		return strings.Fields(stat)
	}
	pid := strings.TrimSpace(stat[:start])
	comm := stat[start : end+1]
	rest := strings.Fields(stat[end+1:])
	result := []string{pid, comm}
	result = append(result, rest...)
	return result
}

func getProcessName(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// ─── Proc Leak ───────────────────────────────────────────────────────────────

type ProcMemInfo struct {
	PID     int
	Name    string
	VmRSS   int // KB
	VmSize  int // KB
	FDs     int
	Threads int
}

func ProcLeak(args []string) error {
	SectionHeader("PROC-LEAK — Memory Leak Detector")
	PrintInfo("Scanning process memory usage...")
	fmt.Println()

	procs, err := collectProcMemInfo()
	if err != nil {
		return fmt.Errorf("failed to read /proc: %v", err)
	}

	sort.Slice(procs, func(i, j int) bool {
		return procs[i].VmRSS > procs[j].VmRSS
	})

	header := color.New(color.FgYellow, color.Bold)
	header.Printf("    %-8s %-22s %10s %10s %8s %8s\n",
		"PID", "NAME", "RSS(MB)", "VIRT(MB)", "FDs", "THREADS")
	fmt.Println("    " + strings.Repeat("─", 72))

	limit := 20
	if len(procs) < limit {
		limit = len(procs)
	}

	for _, p := range procs[:limit] {
		rssMB := float64(p.VmRSS) / 1024.0
		virtMB := float64(p.VmSize) / 1024.0

		lineColor := color.New(color.FgHiWhite)
		if rssMB > 500 {
			lineColor = color.New(color.FgRed, color.Bold)
		} else if rssMB > 200 {
			lineColor = color.New(color.FgYellow)
		}

		lineColor.Printf("    %-8d %-22s %10.1f %10.1f %8d %8d\n",
			p.PID, TruncStr(p.Name, 22), rssMB, virtMB, p.FDs, p.Threads)
	}

	fmt.Println()
	// Total memory usage
	var totalRSS int
	for _, p := range procs {
		totalRSS += p.VmRSS
	}
	PrintKeyVal("Total RSS across all procs", fmt.Sprintf("%.1f MB", float64(totalRSS)/1024.0))

	highMem := 0
	for _, p := range procs {
		if float64(p.VmRSS)/1024.0 > 200 {
			highMem++
		}
	}
	if highMem > 0 {
		PrintWarn("%d process(es) using >200MB RSS — potential memory hogs", highMem)
	} else {
		PrintGood("No obvious memory hogs detected.")
	}

	SectionEnd()
	return nil
}

func collectProcMemInfo() ([]ProcMemInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var procs []ProcMemInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		name := getProcessName(pid)
		status, err := ReadLines(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			continue
		}

		p := ProcMemInfo{PID: pid, Name: name}
		for _, line := range status {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			switch parts[0] {
			case "VmRSS:":
				p.VmRSS = ParseInt(parts[1])
			case "VmSize:":
				p.VmSize = ParseInt(parts[1])
			case "Threads:":
				p.Threads = ParseInt(parts[1])
			}
		}

		// Count FDs
		fdPath := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdPath)
		if err == nil {
			p.FDs = len(fds)
		}

		procs = append(procs, p)
	}
	return procs, nil
}

// ─── Strace Top ───────────────────────────────────────────────────────────────

func StraceTop(args []string) error {
	SectionHeader("STRACE-TOP — Top Syscall Activity")

	if !IsRoot() {
		PrintWarn("strace-top works best with root privileges for full access.")
	}

	PrintInfo("Sampling syscall activity for 3 seconds via /proc/<pid>/syscall...")
	fmt.Println()

	type SyscallStat struct {
		PID  int
		Name string
		Sys  string
	}

	entries, _ := os.ReadDir("/proc")
	var stats []SyscallStat

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/syscall", pid))
		if err != nil {
			continue
		}

		fields := strings.Fields(string(data))
		if len(fields) == 0 || fields[0] == "running" || fields[0] == "-1" {
			continue
		}

		sysNum := fields[0]
		// Map common syscall numbers to names
		sysName := syscallName(sysNum)
		name := getProcessName(pid)

		stats = append(stats, SyscallStat{PID: pid, Name: name, Sys: sysName})
	}

	// Count syscalls per type
	syscallCounts := map[string]int{}
	for _, s := range stats {
		syscallCounts[s.Sys]++
	}

	type kv struct {
		Key string
		Val int
	}
	var sorted []kv
	for k, v := range syscallCounts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Val > sorted[j].Val })

	header := color.New(color.FgYellow, color.Bold)
	header.Printf("    %-30s %s\n", "SYSCALL", "ACTIVE PROCESSES")
	fmt.Println("    " + strings.Repeat("─", 50))

	for i, kv := range sorted {
		if i >= 15 {
			break
		}
		bar := strings.Repeat("█", kv.Val)
		if len(bar) > 20 {
			bar = bar[:20]
		}
		color.New(color.FgCyan).Printf("    %-30s %3d %s\n", kv.Key, kv.Val, bar)
	}

	fmt.Println()
	PrintInfo("Total processes sampled: %d", len(stats))
	PrintInfo("Tip: For live strace, run: strace -c -p <PID>")

	SectionEnd()
	return nil
}

func syscallName(num string) string {
	names := map[string]string{
		"0": "read", "1": "write", "2": "open", "3": "close",
		"4": "stat", "5": "fstat", "6": "lstat", "7": "poll",
		"8": "lseek", "9": "mmap", "10": "mprotect", "11": "munmap",
		"12": "brk", "13": "rt_sigaction", "14": "rt_sigprocmask",
		"17": "pread64", "18": "pwrite64", "19": "readv", "20": "writev",
		"21": "access", "22": "pipe", "23": "select", "24": "sched_yield",
		"35": "nanosleep", "39": "getpid", "41": "socket", "42": "connect",
		"43": "accept", "44": "sendto", "45": "recvfrom", "49": "bind",
		"50": "listen", "202": "futex", "228": "clock_gettime",
		"232": "epoll_wait", "257": "openat", "262": "newfstatat",
	}
	if name, ok := names[num]; ok {
		return name
	}
	return "syscall#" + num
}

// ─── Suspicious Process ───────────────────────────────────────────────────────

func SuspiciousProc(args []string) error {
	SectionHeader("SUSPICIOUS-PROC — Anomaly Detector")
	PrintInfo("Scanning for suspicious processes...")
	fmt.Println()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return fmt.Errorf("cannot read /proc: %v", err)
	}

	type Finding struct {
		PID    int
		Name   string
		Reason string
		Exe    string
	}

	var findings []Finding

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		name := getProcessName(pid)
		exe, _ := filepath.EvalSymlinks(fmt.Sprintf("/proc/%d/exe", pid))
		cmdlineBytes, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")

		var reasons []string

		// Check for deleted executable
		exeLink := fmt.Sprintf("/proc/%d/exe", pid)
		target, err := os.Readlink(exeLink)
		if err == nil && strings.Contains(target, "(deleted)") {
			reasons = append(reasons, "running from deleted binary")
		}

		// Check for processes with no exe (kernel threads excluded)
		if exe == "" && pid > 2 && name != "" && !strings.HasPrefix(name, "k") {
			reasons = append(reasons, "no executable path")
		}

		// Check for unusual exe paths
		if exe != "" {
			suspiciousPaths := []string{"/tmp/", "/dev/shm/", "/run/user/", "/var/tmp/"}
			for _, sp := range suspiciousPaths {
				if strings.HasPrefix(exe, sp) {
					reasons = append(reasons, fmt.Sprintf("running from suspicious path: %s", exe))
					break
				}
			}
		}

		// Check for processes masquerading as system processes
		systemNames := []string{"systemd", "init", "kthreadd", "sshd", "cron"}
		for _, sn := range systemNames {
			if name == sn && exe != "" &&
				!strings.HasPrefix(exe, "/usr/") &&
				!strings.HasPrefix(exe, "/sbin/") &&
				!strings.HasPrefix(exe, "/lib/") {
				reasons = append(reasons, fmt.Sprintf("name '%s' but running from non-standard path", name))
				break
			}
		}

		// Check cmdline for suspicious patterns
		suspiciousPatterns := []string{"nc -e", "bash -i", "/dev/tcp", "base64 -d", "curl|sh", "wget|sh"}
		for _, pat := range suspiciousPatterns {
			if strings.Contains(cmdline, pat) {
				reasons = append(reasons, fmt.Sprintf("suspicious cmdline pattern: %s", pat))
				break
			}
		}

		if len(reasons) > 0 {
			findings = append(findings, Finding{
				PID:    pid,
				Name:   name,
				Reason: strings.Join(reasons, "; "),
				Exe:    exe,
			})
		}
	}

	if len(findings) == 0 {
		PrintGood("No suspicious processes detected.")
		SectionEnd()
		return nil
	}

	color.New(color.FgRed, color.Bold).Printf("  ⚠  Found %d suspicious process(es):\n\n", len(findings))

	for _, f := range findings {
		color.New(color.FgRed, color.Bold).Printf("    PID %-8d  %s\n", f.PID, f.Name)
		color.New(color.FgYellow).Printf("    Reason: %s\n", f.Reason)
		if f.Exe != "" {
			color.New(color.FgHiBlack).Printf("    Exe:    %s\n", f.Exe)
		}
		fmt.Println()
	}

	SectionEnd()
	return nil
}
