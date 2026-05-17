package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fatih/color"
)

// ─── Net Diagnose ─────────────────────────────────────────────────────────────

func NetDiagnose(args []string) error {
	SectionHeader("NET-DIAGNOSE — Network Diagnostics")
	fmt.Println()

	// Network interfaces
	color.New(color.FgCyan, color.Bold).Println("  ● Network Interfaces")
	out, err := RunShell("ip -o addr show 2>/dev/null || ifconfig 2>/dev/null")
	if err == nil && out != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			// Highlight UP/DOWN status
			if strings.Contains(line, "UP") {
				color.New(color.FgGreen).Printf("    %s\n", TruncStr(line, 100))
			} else if strings.Contains(line, "DOWN") {
				color.New(color.FgRed).Printf("    %s\n", TruncStr(line, 100))
			} else {
				color.New(color.FgHiBlack).Printf("    %s\n", TruncStr(line, 100))
			}
		}
	}

	fmt.Println()

	// Default route
	color.New(color.FgCyan, color.Bold).Println("  ● Default Route")
	out, _ = RunShell("ip route show default 2>/dev/null || route -n 2>/dev/null | head -5")
	if strings.TrimSpace(out) != "" {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			color.New(color.FgHiWhite).Printf("    %s\n", line)
		}
	} else {
		PrintWarn("No default route found!")
	}

	fmt.Println()

	// DNS resolution
	color.New(color.FgCyan, color.Bold).Println("  ● DNS Servers (/etc/resolv.conf)")
	lines, _ := ReadLines("/etc/resolv.conf")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		color.New(color.FgHiWhite).Printf("    %s\n", line)
	}

	fmt.Println()

	// Connectivity checks
	color.New(color.FgCyan, color.Bold).Println("  ● Connectivity Tests")
	targets := []struct{ host, desc string }{
		{"8.8.8.8", "Google DNS (IPv4)"},
		{"1.1.1.1", "Cloudflare DNS"},
		{"google.com", "DNS resolution"},
	}

	for _, t := range targets {
		out, err := RunShell(fmt.Sprintf("ping -c 1 -W 2 %s 2>&1 | tail -2", t.host))
		if err == nil && strings.Contains(out, "1 received") {
			PrintGood("%-30s ✓ Reachable", t.desc)
		} else if strings.Contains(out, "rtt") {
			PrintGood("%-30s ✓ Reachable", t.desc)
		} else {
			PrintBad("%-30s ✗ Unreachable", t.desc)
		}
	}

	fmt.Println()

	// Listening ports
	color.New(color.FgCyan, color.Bold).Println("  ● Listening Ports")
	out, _ = RunShell("ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null")
	if strings.TrimSpace(out) != "" {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		for i, line := range lines {
			if i == 0 {
				color.New(color.FgYellow, color.Bold).Printf("    %s\n", line)
			} else {
				color.New(color.FgHiWhite).Printf("    %s\n", TruncStr(line, 110))
			}
		}
	}

	SectionEnd()
	return nil
}

// ─── TCP Bottleneck ───────────────────────────────────────────────────────────

type TcpConn struct {
	LocalAddr  string
	RemoteAddr string
	State      string
	RecvQ      int
	SendQ      int
	Process    string
}

func TcpBottleneck(args []string) error {
	SectionHeader("TCP-BOTTLENECK — TCP Congestion Analyzer")
	PrintInfo("Analyzing TCP connections for bottlenecks...")
	fmt.Println()

	// Get connections with queues
	out, err := RunShell("ss -tnp 2>/dev/null")
	if err != nil {
		PrintWarn("ss command not available, trying netstat...")
		out, err = RunShell("netstat -tnp 2>/dev/null")
		if err != nil {
			return fmt.Errorf("neither ss nor netstat available")
		}
	}

	var conns []TcpConn
	lines := strings.Split(out, "\n")

	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}

		conn := TcpConn{
			RecvQ: ParseInt(parts[1]),
			SendQ: ParseInt(parts[2]),
		}

		// Handle ss output format
		if len(parts) >= 5 {
			conn.State = parts[0]
			conn.LocalAddr = parts[3]
			conn.RemoteAddr = parts[4]
		}
		if len(parts) >= 6 {
			conn.Process = parts[5]
		}

		conns = append(conns, conn)
	}

	// Sort by RecvQ + SendQ descending
	sort.Slice(conns, func(i, j int) bool {
		return (conns[i].RecvQ + conns[i].SendQ) > (conns[j].RecvQ + conns[j].SendQ)
	})

	// Show bottlenecks
	var bottlenecks []TcpConn
	for _, c := range conns {
		if c.RecvQ > 0 || c.SendQ > 0 {
			bottlenecks = append(bottlenecks, c)
		}
	}

	if len(bottlenecks) == 0 {
		PrintGood("No TCP queue backlog detected. All connections flowing normally.")
	} else {
		color.New(color.FgRed, color.Bold).Printf("  ⚠  %d connections with queue backlog:\n\n", len(bottlenecks))

		header := color.New(color.FgYellow, color.Bold)
		header.Printf("    %-12s %8s %8s %-25s %-25s\n",
			"STATE", "RECV-Q", "SEND-Q", "LOCAL", "REMOTE")
		fmt.Println("    " + strings.Repeat("─", 82))

		for _, c := range bottlenecks {
			lineColor := color.New(color.FgRed, color.Bold)
			if c.RecvQ+c.SendQ < 100 {
				lineColor = color.New(color.FgYellow)
			}
			lineColor.Printf("    %-12s %8d %8d %-25s %-25s\n",
				c.State, c.RecvQ, c.SendQ,
				TruncStr(c.LocalAddr, 25), TruncStr(c.RemoteAddr, 25))
		}
	}

	fmt.Println()

	// TCP stats from /proc
	color.New(color.FgCyan, color.Bold).Println("  ● TCP Statistics")
	tcpStats, _ := ReadLines("/proc/net/snmp")
	var tcpKeys, tcpVals []string
	for _, line := range tcpStats {
		if strings.HasPrefix(line, "Tcp:") {
			parts := strings.Fields(line)
			if len(tcpKeys) == 0 {
				tcpKeys = parts[1:]
			} else {
				tcpVals = parts[1:]
			}
		}
	}

	interestingKeys := map[string]bool{
		"RetransSegs": true, "InErrs": true, "OutRsts": true,
		"AttemptFails": true, "EstabResets": true, "CurrEstab": true,
	}

	for i, k := range tcpKeys {
		if !interestingKeys[k] {
			continue
		}
		if i < len(tcpVals) {
			val := tcpVals[i]
			lineColor := color.New(color.FgHiWhite)
			if (k == "RetransSegs" || k == "InErrs") && ParseInt(val) > 1000 {
				lineColor = color.New(color.FgYellow)
			}
			lineColor.Printf("    %-20s %s\n", k+":", val)
		}
	}

	SectionEnd()
	return nil
}

// ─── Conn Tracker ─────────────────────────────────────────────────────────────

func ConnTracker(args []string) error {
	SectionHeader("CONN-TRACKER — Connection State Summary")
	PrintInfo("Enumerating active connections by state...")
	fmt.Println()

	out, err := RunShell("ss -tan 2>/dev/null || netstat -tan 2>/dev/null")
	if err != nil {
		return fmt.Errorf("cannot list connections: %v", err)
	}

	// Count by state
	stateCounts := map[string]int{}
	remoteHosts := map[string]int{}
	localPorts := map[string]int{}

	lines := strings.Split(out, "\n")
	total := 0
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}

		state := parts[0]
		// For ss output, state is first field
		stateCounts[state]++
		total++

		if len(parts) > 4 {
			remote := parts[4]
			if remote != "*:*" && remote != "0.0.0.0:*" {
				// Extract host only
				if idx := strings.LastIndex(remote, ":"); idx > 0 {
					host := remote[:idx]
					remoteHosts[host]++
				}
			}
			local := parts[3]
			if idx := strings.LastIndex(local, ":"); idx > 0 {
				port := local[idx+1:]
				localPorts[port]++
			}
		}
	}

	// Print state summary
	color.New(color.FgCyan, color.Bold).Println("  ● Connections by State")
	header := color.New(color.FgYellow, color.Bold)
	header.Printf("    %-25s %8s  %s\n", "STATE", "COUNT", "VISUAL")
	fmt.Println("    " + strings.Repeat("─", 60))

	stateOrder := []string{"ESTAB", "ESTABLISHED", "TIME-WAIT", "CLOSE-WAIT",
		"LISTEN", "SYN-SENT", "SYN-RECV", "FIN-WAIT-1", "FIN-WAIT-2", "LAST-ACK", "CLOSED"}

	for _, state := range stateOrder {
		count, ok := stateCounts[state]
		if !ok {
			continue
		}
		bar := strings.Repeat("█", count)
		if len(bar) > 30 {
			bar = bar[:30] + "+"
		}
		lineColor := color.New(color.FgHiWhite)
		switch {
		case state == "ESTAB" || state == "ESTABLISHED":
			lineColor = color.New(color.FgGreen)
		case state == "TIME-WAIT":
			lineColor = color.New(color.FgYellow)
		case state == "CLOSE-WAIT":
			lineColor = color.New(color.FgRed)
		case state == "LISTEN":
			lineColor = color.New(color.FgCyan)
		}
		lineColor.Printf("    %-25s %8d  %s\n", state, count, bar)
	}

	// Print others
	for state, count := range stateCounts {
		found := false
		for _, s := range stateOrder {
			if s == state {
				found = true
				break
			}
		}
		if !found {
			color.New(color.FgHiBlack).Printf("    %-25s %8d\n", state, count)
		}
	}

	fmt.Println()
	PrintKeyVal("Total connections", fmt.Sprintf("%d", total))

	// Top remote hosts
	fmt.Println()
	color.New(color.FgCyan, color.Bold).Println("  ● Top Remote Hosts")

	type kv struct{ K string; V int }
	var sorted []kv
	for k, v := range remoteHosts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].V > sorted[j].V })

	for i, kv := range sorted {
		if i >= 8 {
			break
		}
		color.New(color.FgHiWhite).Printf("    %-30s %d connections\n", kv.K, kv.V)
	}

	// TIME_WAIT warning
	if tw, ok := stateCounts["TIME-WAIT"]; ok && tw > 1000 {
		fmt.Println()
		PrintWarn("High TIME-WAIT count (%d). Consider tuning net.ipv4.tcp_tw_reuse", tw)
	}

	if cw, ok := stateCounts["CLOSE-WAIT"]; ok && cw > 100 {
		fmt.Println()
		PrintBad("High CLOSE-WAIT count (%d). Likely application-level socket leak!", cw)
	}

	SectionEnd()
	return nil
}
