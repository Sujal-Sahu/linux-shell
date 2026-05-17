// Linux Management Shell
// A CLI-based Linux system management and debugging toolkit

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
)

const banner = `
██╗     ██╗███╗   ██╗██╗   ██╗██╗  ██╗███████╗██╗  ██╗███████╗██╗     ██╗
██║     ██║████╗  ██║██║   ██║╚██╗██╔╝██╔════╝██║  ██║██╔════╝██║     ██║
██║     ██║██╔██╗ ██║██║   ██║ ╚███╔╝ ███████╗███████║█████╗  ██║     ██║
██║     ██║██║╚██╗██║██║   ██║ ██╔██╗ ╚════██║██╔══██║██╔══╝  ██║     ██║
███████╗██║██║ ╚████║╚██████╔╝██╔╝ ██╗███████║██║  ██║███████╗███████╗███████╗
╚══════╝╚═╝╚═╝  ╚═══╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝
`

type Command struct {
	Name        string
	Description string
	Category    string
	Handler     func(args []string) error
}

var commands map[string]*Command

func init() {
	commands = map[string]*Command{

		// ── DEBUG: Process ────────────────────────────────────────────────
		"zombie-hunter": {
			Name: "zombie-hunter", Description: "Find and kill zombie (defunct) processes",
			Category: "Debug: Process", Handler: ZombieHunter,
		},
		"proc-leak": {
			Name: "proc-leak", Description: "Detect memory leaks and high memory consuming processes",
			Category: "Debug: Process", Handler: ProcLeak,
		},
		"strace-top": {
			Name: "strace-top", Description: "Show top system calls per process (requires root)",
			Category: "Debug: Process", Handler: StraceTop,
		},
		"suspicious-proc": {
			Name: "suspicious-proc", Description: "Detect suspicious processes (hidden, unusual paths)",
			Category: "Debug: Process", Handler: SuspiciousProc,
		},

		// ── DEBUG: Memory ─────────────────────────────────────────────────
		"oom-killer": {
			Name: "oom-killer", Description: "Show OOM killer candidates and interactively free memory",
			Category: "Debug: Memory", Handler: OomKiller,
		},
		"oom-history": {
			Name: "oom-history", Description: "Show historical OOM killer events from system logs",
			Category: "Debug: Memory", Handler: OomHistory,
		},

		// ── DEBUG: Network ────────────────────────────────────────────────
		"net-diagnose": {
			Name: "net-diagnose", Description: "Diagnose network interfaces, routes, and connectivity",
			Category: "Debug: Network", Handler: NetDiagnose,
		},
		"tcp-bottleneck": {
			Name: "tcp-bottleneck", Description: "Identify TCP connections causing bottlenecks",
			Category: "Debug: Network", Handler: TcpBottleneck,
		},
		"conn-tracker": {
			Name: "conn-tracker", Description: "Track and summarize active network connections by state",
			Category: "Debug: Network", Handler: ConnTracker,
		},

		// ── DEBUG: Disk ───────────────────────────────────────────────────
		"disk-health": {
			Name: "disk-health", Description: "Check disk usage, I/O stats and identify disk pressure",
			Category: "Debug: Disk", Handler: DiskHealth,
		},
		"inode-crisis": {
			Name: "inode-crisis", Description: "Detect inode exhaustion across mounted filesystems",
			Category: "Debug: Disk", Handler: InodeCrisis,
		},
		"fsck-report": {
			Name: "fsck-report", Description: "Generate a filesystem check report from system logs",
			Category: "Debug: Disk", Handler: FsckReport,
		},

		// ── DEBUG: System ─────────────────────────────────────────────────
		"service-trace": {
			Name: "service-trace", Description: "Trace systemd service status, logs and dependencies",
			Category: "Debug: System", Handler: ServiceTrace,
		},
		"cron-debug": {
			Name: "cron-debug", Description: "Debug cron jobs: list, validate and check recent executions",
			Category: "Debug: System", Handler: CronDebug,
		},
		"boot-timeline": {
			Name: "boot-timeline", Description: "Display system boot timeline with service start times",
			Category: "Debug: System", Handler: BootTimeline,
		},
		"recent-logins": {
			Name: "recent-logins", Description: "Show recent login history including failed attempts",
			Category: "Debug: System", Handler: RecentLogins,
		},

		// ── MANAGE: Services ──────────────────────────────────────────────
		"service": {
			Name: "service", Description: "Create/list/start/stop/restart/delete/edit/logs systemd services",
			Category: "Manage: Services", Handler: ServiceManager,
		},

		// ── MANAGE: Jobs (Cron) ───────────────────────────────────────────
		"job": {
			Name: "job", Description: "Create/list/run/enable/disable/delete scheduled jobs (systemd timers)",
			Category: "Manage: Jobs", Handler: JobManager,
		},

		// ── MANAGE: Rollback ──────────────────────────────────────────────
		"rollback": {
			Name: "rollback", Description: "List and restore any config changed by linuxshell",
			Category: "Manage: Rollback", Handler: RollbackManager,
		},
	}
}

func printBanner() {
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Println(banner)
	yellow := color.New(color.FgYellow)
	yellow.Println("  🐧 Linux Management Shell — Type 'help' to list commands, 'exit' to quit\n")
}

func printHelp() {
	categories := map[string][]*Command{}
	for _, cmd := range commands {
		categories[cmd.Category] = append(categories[cmd.Category], cmd)
	}

	catOrder := []string{
		"Manage: Services", "Manage: Jobs", "Manage: Rollback",
		"Debug: Process", "Debug: Memory", "Debug: Network", "Debug: Disk", "Debug: System",
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	bold := color.New(color.FgWhite, color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	dim := color.New(color.FgHiBlack)

	fmt.Println()
	for _, cat := range catOrder {
		cmds, ok := categories[cat]
		if !ok {
			continue
		}
		bold.Printf("  ── %s ──\n", cat)
		sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
		for _, cmd := range cmds {
			green.Fprintf(w, "    %s\t", cmd.Name)
			dim.Fprintf(w, "%s\n", cmd.Description)
		}
		w.Flush()
		fmt.Println()
	}

	fmt.Println()
	bold.Println("  Built-in commands:")
	fmt.Fprintf(w, "    help\tShow this help message\n")
	fmt.Fprintf(w, "    clear\tClear the terminal screen\n")
	fmt.Fprintf(w, "    exit\tExit DebugShell\n")
	w.Flush()
	fmt.Println()
}

func getCommandNames() []string {
	names := []string{"help", "clear", "exit"}
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func main() {
	printBanner()

	completer := readline.NewPrefixCompleter(
		func() []readline.PrefixCompleterInterface {
			items := []readline.PrefixCompleterInterface{}
			for _, name := range getCommandNames() {
				items = append(items, readline.PcItem(name))
			}
			return items
		}()...,
	)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[36mlinuxshell\033[0m \033[33m»\033[0m ",
		HistoryFile:     "/tmp/.linuxshell_history",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing shell: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	for {
		line, err := rl.Readline()
		if err != nil {
			fmt.Println("\nGoodbye!")
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmdName := parts[0]
		args := parts[1:]

		switch cmdName {
		case "exit", "quit":
			yellow.Println("Goodbye! Keep your systems healthy. 🐧")
			return
		case "help":
			printHelp()
		case "clear":
			fmt.Print("\033[H\033[2J")
			printBanner()
		default:
			cmd, ok := commands[cmdName]
			if !ok {
				red.Printf("  Unknown command: '%s'. Type 'help' to see available commands.\n\n", cmdName)
				continue
			}
			fmt.Println()
			if err := cmd.Handler(args); err != nil {
				red.Printf("  Error: %v\n", err)
			}
			fmt.Println()
		}
	}
}
