# 🐧 LinuxShell

**An interactive Linux management and debugging shell written in Go.**

LinuxShell is a unified shell that combines two capabilities in one binary — a **management plane** for creating and operating systemd services and scheduled jobs, and a **debugging toolkit** of 16 diagnostic utilities for deep Linux introspection. Inspired by NVIDIA BCM's declarative object model, every service and job is defined through a structured wizard, previewed before deployment, and automatically backed up before any change.

```
linuxshell »  service create      # define a full systemd service interactively
linuxshell »  job create          # schedule a job with OnCalendar expressions
linuxshell »  zombie-hunter       # find and kill zombie processes
linuxshell »  oom-killer          # see what the kernel would kill next
```

---

## Features

- **Service Manager** — full CRUD for systemd services via an attribute wizard. Defines every `[Unit]`, `[Service]`, and `[Install]` directive including resource limits, environment files, and restart policy. Previews the unit file before writing it.
- **Job Manager** — create and manage scheduled jobs backed by systemd timers. Supports `OnCalendar` expressions, boot delays, persistence across missed runs, file or journal logging, and resource limits per job.
- **Automatic Backup & Rollback** — every create, edit, or delete backs up the original file to `~/.linuxshell/backups/` and logs the change. Restore any file with a single command.
- **16 Diagnostic Utilities** — covering processes, memory, network, disk, and system, all reading directly from `/proc/`, kernel logs, and standard Linux interfaces.
- **Tab-completion** — all commands and sub-commands complete on `Tab`.
- **Command history** — persisted across sessions in `/tmp/.linuxshell_history`.
- **Confirmation prompts** — every destructive action (delete, kill, overwrite) requires explicit `y` before proceeding.
- **Colored output** — severity-coded: green = OK, yellow = warning, red = critical.

---

## Requirements

- Linux (Ubuntu 20.04+, Debian 11+, CentOS 7+, or any systemd-based distro)
- Go 1.21 or later
- `systemd` (for `service` and `job` commands)
- Root / sudo recommended (some debug commands read `/proc` entries owned by other users)

---

## Installation

```bash
# Clone
git clone https://github.com/yourname/linuxshell.git
cd linuxshell

# Download dependencies and build
go mod tidy
go build -o linuxshell .

# Run (sudo gives full /proc access)
sudo ./linuxshell
```

Or install system-wide:

```bash
sudo cp linuxshell /usr/local/bin/linuxshell
linuxshell
```

---

## Quick Start

```
linuxshell »  help               # list all commands grouped by category
linuxshell »  service            # service sub-command help
linuxshell »  job                # job sub-command help
linuxshell »  [TAB]              # tab-complete any command
linuxshell »  clear              # clear screen
linuxshell »  exit               # quit
```

---

## Management Commands

### `service` — Systemd Service Manager

Create, inspect, control, and remove systemd services without touching unit files manually.

```
service create             Wizard: fill in every attribute, preview the unit, then deploy
service list               All services with load state and active state
service status  <name>     Full status: uptime, PID, memory, recent logs
service start   <name>     Start a service
service stop    <name>     Stop a service
service restart <name>     Restart a service
service reload  <name>     Reload config without restart (requires ExecReload to be set)
service enable  <name>     Enable to start at boot
service disable <name>     Disable from starting at boot
service logs    <name> [n] Last N journal lines, coloured by severity (default 50)
service show    <name>     Raw unit properties dump (equivalent to systemctl show)
service edit    <name>     Open unit file in $EDITOR — auto-backup before opening
service delete  <name>     Stop → disable → remove unit file (backup is kept)
service export  <name>     Write full definition to <name>.svc.json
service import  <file>     Re-create a service from an exported JSON file
```

#### Service Object — attributes defined in the wizard

The wizard walks through every attribute grouped by unit section:

**[Unit]**

| Attribute | Systemd Directive | Example |
|---|---|---|
| Name | — | `api-server` |
| Description | `Description=` | `My API Server` |
| After | `After=` | `network.target` |
| Requires | `Requires=` | `postgresql.service` |
| Wants | `Wants=` | `redis.service` |

**[Service]**

| Attribute | Systemd Directive | Options / Example |
|---|---|---|
| Type | `Type=` | `simple` \| `forking` \| `oneshot` \| `notify` \| `dbus` |
| ExecStart | `ExecStart=` | `/usr/bin/myapp --port 8080` |
| ExecStop | `ExecStop=` | `/usr/bin/myapp stop` |
| ExecReload | `ExecReload=` | `/bin/kill -HUP $MAINPID` |
| WorkingDirectory | `WorkingDirectory=` | `/opt/myapp` |
| User | `User=` | `appuser` |
| Group | `Group=` | `appgroup` |
| Restart | `Restart=` | `no` \| `on-failure` \| `always` \| `on-abnormal` |
| RestartSec | `RestartSec=` | `5` |
| Environment | `Environment=` | `PORT=8080 DEBUG=false` |
| EnvironmentFile | `EnvironmentFile=` | `/etc/myapp/env` |
| CPUQuota | `CPUQuota=` | `50%` (half a core), `200%` (two cores) |
| MemoryMax | `MemoryMax=` | `512M`, `2G` |
| TasksMax | `TasksMax=` | `64` |
| LimitNOFILE | `LimitNOFILE=` | `65536` |
| StandardOutput | `StandardOutput=` | `journal` \| `syslog` \| `null` \| `file:/var/log/app.log` |
| StandardError | `StandardError=` | `journal` \| `inherit` \| `null` |
| TimeoutStartSec | `TimeoutStartSec=` | `90` |
| TimeoutStopSec | `TimeoutStopSec=` | `30` |

**[Install]**

| Attribute | Systemd Directive | Options |
|---|---|---|
| WantedBy | `WantedBy=` | `multi-user.target` \| `graphical.target` \| `network-online.target` |

#### Example session

```
linuxshell »  service create

  ┌─ Fill in every attribute. Press Enter to accept the [default]. Ctrl+C to abort.
  │
  │  [Unit]
    Service name (no .service suffix): api-server
    Description           [api-server managed service]: My API Server
    After                 [network.target]:
    Requires              (hard deps, optional):
    Wants                 (soft deps, optional): postgresql.service
  │
  │  [Service]
    Type                  [simple]:
    ExecStart             : /opt/api/server --port 8080
    ExecReload            (optional): /bin/kill -HUP $MAINPID
    WorkingDirectory      (optional): /opt/api
    User                  [root]: appuser
    Restart               [on-failure]:
    RestartSec            [5]:
    Environment           (optional): PORT=8080 ENV=production
    EnvironmentFile       (optional): /etc/api/env
  │
  │  [Service] — Resource limits
    CPUQuota              (e.g. 50%): 80%
    MemoryMax             (e.g. 512M, 2G): 1G
    LimitNOFILE           : 65536
  │
  │  [Service] — Logging & Timeouts
    StandardOutput        [journal]:
    StandardError         [journal]:
    TimeoutStartSec       [90]:
    TimeoutStopSec        [30]:
  │
  │  [Install]
    WantedBy              [multi-user.target]:

  └──────────────────────────────────────────────────────

  ● Preview — /etc/systemd/system/api-server.service
    ────────────────────────────────────────────────────────
    [Unit]
    Description=My API Server
    After=network.target
    Wants=postgresql.service

    [Service]
    Type=simple
    ExecStart=/opt/api/server --port 8080
    ExecReload=/bin/kill -HUP $MAINPID
    WorkingDirectory=/opt/api
    User=appuser
    Restart=on-failure
    RestartSec=5
    Environment=PORT=8080 ENV=production
    EnvironmentFile=/etc/api/env
    CPUQuota=80%
    MemoryMax=1G
    LimitNOFILE=65536
    StandardOutput=journal
    StandardError=journal
    TimeoutStartSec=90
    TimeoutStopSec=30

    [Install]
    WantedBy=multi-user.target
    ────────────────────────────────────────────────────────

  ⚠  Write, enable and start 'api-server.service'? [y/N]: y

  → Reload systemd daemon...                       OK
  → Enable at boot...                              OK
  → Start service...                               OK

  ● api-server.service - My API Server
       Loaded: loaded (/etc/systemd/system/api-server.service; enabled)
       Active: active (running) since Thu 2025-01-15 14:30:22 UTC
```

---

### `job` — Scheduled Job Manager

Create and manage scheduled jobs using systemd timers. Each job creates two units — a `.timer` that drives the schedule and a `-job.service` that runs the command.

```
job create             Wizard: define every attribute, preview both units, then deploy
job list               All timers: last run, next run, associated service
job status  <name>     Timer state + service state + stored job definition
job run     <name>     Trigger the job immediately, bypassing the schedule
job logs    <name> [n] Last N journal lines from the job service (default 50)
job enable  <name>     Enable and start the timer
job disable <name>     Disable and stop the timer (definition stays on disk)
job delete  <name>     Remove both .timer and -job.service units (backup kept)
job validate <name>    Check schedule syntax, verify command binary exists and is executable
job schedules          OnCalendar expression reference with examples
```

#### Job Object — attributes defined in the wizard

**Identity**

| Attribute | Description |
|---|---|
| Name | Becomes `<name>.timer` and `<name>-job.service` |
| Description | Human-readable description |

**What to run**

| Attribute | Systemd Directive | Example |
|---|---|---|
| Command | `ExecStart=` | `/opt/scripts/backup.sh --db postgres` |
| User | `User=` | `backup` |
| WorkingDirectory | `WorkingDirectory=` | `/opt/scripts` |
| Environment | `Environment=` | `DEST=/mnt/backup LOG=verbose` |
| EnvironmentFile | `EnvironmentFile=` | `/etc/backup/env` |

**When to run**

| Attribute | Systemd Directive | Example |
|---|---|---|
| Schedule | `OnCalendar=` | `daily`, `*:0/15`, `Mon..Fri *-*-* 08:00:00` |
| OnBootSec | `OnBootSec=` | `5min` — also run 5 minutes after every boot |
| Persistent | `Persistent=` | `yes` — run any executions missed while system was off |

**Output**

| Option | Effect |
|---|---|
| `journal` | `StandardOutput=journal` — view with `job logs <name>` |
| `file` | `StandardOutput=append:/var/log/<name>-job.log` |

**Resource limits**

| Attribute | Systemd Directive | Example |
|---|---|---|
| CPUQuota | `CPUQuota=` | `50%` |
| MemoryMax | `MemoryMax=` | `256M` |

#### OnCalendar expression reference

```
minutely                         Every minute
hourly                           Every hour at :00
daily                            Every day at midnight
weekly                           Every Monday at midnight
monthly                          1st of every month at midnight
*:0/5                            Every 5 minutes
*:0/15                           Every 15 minutes
*:0/30                           Every 30 minutes
*-*-* 02:00:00                   Every day at 2:00 AM
*-*-* 09:30:00                   Every day at 9:30 AM
Mon..Fri *-*-* 08:00:00          Weekdays at 8:00 AM
Sat,Sun *-*-* 10:00:00           Weekends at 10:00 AM
Mon *-*-* 03:00:00               Every Monday at 3:00 AM
*-*-1 00:00:00                   1st of every month
*-01,07-01 00:00:00              Jan 1 and Jul 1
```

Validate any expression before using it:

```bash
systemd-analyze calendar 'Mon..Fri *-*-* 08:00:00'
```

#### Example session

```
linuxshell »  job create

    Job name (no .timer suffix)      : db-backup
    Description                      [db-backup scheduled job]: Database backup
    Command  (full path + arguments) : /opt/scripts/backup.sh --db postgres
    Run as user                      [root]: backup
    WorkingDirectory (optional)      : /opt/scripts
    Environment (optional)           : PG_HOST=localhost PG_PORT=5432
    EnvironmentFile (optional)       :
    OnCalendar expression            [daily]: *-*-* 02:30:00
    OnBootSec (optional)             : 5min
    Persistent                       [yes]:
    Log output to                    [journal]: file
    Log file path                    [/var/log/db-backup-job.log]:
    CPUQuota (optional)              : 30%
    MemoryMax (optional)             : 256M

  → Validating schedule '*-*-* 02:30:00'...  valid ✓
      Normalized form: *-*-* 02:30:00
      Next elapse:     Thu 2025-01-16 02:30:00 UTC

  ● Preview — /etc/systemd/system/db-backup.timer
    ────────────────────────────────────────────────────────
    [Unit]
    Description=Database backup (timer)

    [Timer]
    OnCalendar=*-*-* 02:30:00
    OnBootSec=5min
    Persistent=true
    Unit=db-backup-job.service

    [Install]
    WantedBy=timers.target
    ────────────────────────────────────────────────────────

  ● Preview — /etc/systemd/system/db-backup-job.service
    ────────────────────────────────────────────────────────
    [Unit]
    Description=Database backup

    [Service]
    Type=oneshot
    ExecStart=/opt/scripts/backup.sh --db postgres
    User=backup
    WorkingDirectory=/opt/scripts
    Environment=PG_HOST=localhost PG_PORT=5432
    CPUQuota=30%
    MemoryMax=256M
    StandardOutput=append:/var/log/db-backup-job.log
    StandardError=append:/var/log/db-backup-job.log
    ────────────────────────────────────────────────────────

  ⚠  Create and enable job 'db-backup'? [y/N]: y

  → Reload systemd daemon...                       OK
  → Enable timer...                                OK
  → Start timer...                                 OK

      Active: active (waiting)
      Trigger: Thu 2025-01-16 02:30:00 UTC
      Triggers: db-backup-job.service
```

---

### `rollback` — Change History and Restore

Every management action that modifies a system file backs up the original automatically. The rollback command gives you a full audit trail and one-command restore.

```
rollback list           Show all tracked changes with IDs and timestamps
rollback apply <id>     Restore the backed-up file to its original path
rollback clear          Wipe the history log (backup files on disk are kept)
```

Backup files are stored at `/root/.linuxshell/backups/`. History is stored at `/root/.linuxshell/history.json`.

```
linuxshell »  rollback list

    ID                                     ACTION   TYPE     NAME           TIMESTAMP
    ─────────────────────────────────────────────────────────────────────────────────
    service-api-server-20250115-143022     create   service  api-server     2025-01-15 14:30:22  ✓ restorable
    job-db-backup-20250115-143511          create   job      db-backup      2025-01-15 14:35:11  ✓ restorable
    service-api-server-20250115-150000     edit     service  api-server     2025-01-15 15:00:00  ✓ restorable

linuxshell »  rollback apply service-api-server-20250115-143022

    Restoring '/etc/systemd/system/api-server.service' from backup...
  ✓  Restored successfully.
  ✓  systemd daemon reloaded.
```

---

## Debug Commands

16 read-only diagnostic utilities that read directly from `/proc/`, kernel logs, and system interfaces — no agents, no daemons required.

### Process

| Command | What it does |
|---|---|
| `zombie-hunter` | Scans `/proc/*/stat` for state `Z`. Shows zombie PID, PPID, and parent name. Offers to SIGKILL the parent to reap the zombie. |
| `proc-leak` | Reads `VmRSS`, `VmSize`, and `Threads` from `/proc/*/status`. Sorts by physical RAM. Flags processes over 200 MB. Shows open file descriptor count per process. |
| `strace-top` | Reads `/proc/*/syscall` across all processes to sample which syscalls are active system-wide without attaching strace to any individual process. |
| `suspicious-proc` | Detects processes running from deleted binaries, staging paths (`/tmp/`, `/dev/shm/`), name-spoofing system binaries, or cmdlines with reverse shell patterns. |

### Memory

| Command | What it does |
|---|---|
| `oom-killer` | Reads `/proc/meminfo` and `/proc/*/oom_score`. Sorts by OOM score — shows exactly what the kernel would kill first. Offers to preemptively kill the top candidate. |
| `oom-history` | Searches `dmesg`, `journalctl -k`, `/var/log/kern.log`, `/var/log/syslog` for past OOM kill events. Highlights kill events red, trigger events yellow. |

### Network

| Command | What it does |
|---|---|
| `net-diagnose` | Checks interfaces, default route, DNS config, pings 8.8.8.8/1.1.1.1/google.com, and lists listening ports. Tests every layer independently. |
| `tcp-bottleneck` | Uses `ss -tnp` to find connections with non-zero `RECV-Q` or `SEND-Q`. Reads `/proc/net/snmp` for TCP retransmission and error stats. |
| `conn-tracker` | Counts all TCP connections by state and renders a bar chart. Warns on high `TIME-WAIT` (tune `tcp_tw_reuse`) and `CLOSE-WAIT` (socket leak in application). |

### Disk

| Command | What it does |
|---|---|
| `disk-health` | `df -hT` coloured by usage percent, top 10 directories by size, I/O stats from `iostat` or `/proc/diskstats`. |
| `inode-crisis` | `df -i` across all mounts sorted by inode usage. Flags over 80% as WARNING, over 95% as CRITICAL. Shows remediation commands inline. |
| `fsck-report` | Greps `dmesg` for filesystem errors, checks boot fsck logs, shows mounted filesystems, queries `smartctl -H` for drive health. |

### System

| Command | What it does |
|---|---|
| `service-trace` | Shows system state, lists failed units, recent events. With a name: full status, last 20 log lines, and dependency tree. |
| `cron-debug` | Reads `/etc/crontab`, `/etc/cron.d/*`, and per-user crontabs. Shows recent execution log entries. |
| `boot-timeline` | `systemd-analyze` for total boot time, `blame` for slowest services, `critical-chain` for the dependency path that determined total boot time. |
| `recent-logins` | Successful logins via `last`, failed attempts via `lastb`/auth.log, active sessions via `who`/`w`, top attacking IPs by failed attempt count. |

---

## Project Structure

```
linuxshell/
├── main.go             Shell REPL, banner, readline setup, command registry
├── utils.go            Shared helpers: RunShell, Confirm, SectionHeader, PrintGood, etc.
├── manage_common.go    Storage paths, BackupFile, RollbackManager, wizard prompt helpers
├── manage_service.go   ServiceObject, ServiceManager and all svc* sub-commands
├── manage_job.go       JobObject, JobManager and all job* sub-commands
├── cmd_process.go      zombie-hunter, proc-leak, strace-top, suspicious-proc
├── cmd_memory.go       oom-killer, oom-history
├── cmd_network.go      net-diagnose, tcp-bottleneck, conn-tracker
├── cmd_disk.go         disk-health, inode-crisis, fsck-report
├── cmd_services.go     service-trace, cron-debug
├── cmd_system.go       boot-timeline, recent-logins
└── go.mod
```

---

## Adding a New Command

The shell uses a dispatch table — adding a command is two steps.

**Step 1** — write a handler in any `.go` file:

```go
func MyCommand(args []string) error {
    SectionHeader("MY-COMMAND — What it does")

    PrintGood("Everything looks fine.")
    PrintWarn("Something needs attention.")
    PrintBad("Something is broken.")
    PrintKeyVal("Key", "value")

    SectionEnd()
    return nil
}
```

**Step 2** — register it in `init()` in `main.go`:

```go
"my-command": {
    Name:        "my-command",
    Description: "Brief description shown in help",
    Category:    "Debug: Process",  // or Manage: Services, etc.
    Handler:     MyCommand,
},
```

Tab-completion and help listing are automatic — nothing else to change.

---

## State Layout

```
/root/.linuxshell/
├── history.json          Audit log of all management changes (JSON)
├── backups/              Copies of files before they were modified
│   ├── service-nginx-20250115-143022.service
│   ├── job-backup-20250115-143511.timer
│   └── ...
├── services/             Full JSON export of every service created via wizard
│   └── api-server.json
└── jobs/                 Full JSON metadata for every job created via wizard
    └── db-backup.json
```

---

## Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/chzyer/readline` | v1.5.1 | Interactive shell: tab-completion, history, Ctrl+C |
| `github.com/fatih/color` | v1.16.0 | ANSI terminal colors and bold/dim text |

Install with `go mod tidy`.

---

## Permissions Reference

| Command | Root needed? | Reason |
|---|---|---|
| `service create / delete / edit` | Yes | Writes to `/etc/systemd/system/`, calls `systemctl` |
| `service start / stop / restart` | Yes | Controls system services |
| `job create / delete` | Yes | Writes to `/etc/systemd/system/`, calls `systemctl` |
| `rollback apply` | Yes | Overwrites files in `/etc/systemd/system/` |
| `zombie-hunter` (kill action) | Yes | Sends signal to another user's process |
| `oom-killer` (kill action) | Yes | Sends signal to another process |
| `strace-top` | Recommended | Reads `/proc/*/syscall` for all users' processes |
| `suspicious-proc` | Recommended | Reads `/proc/*/exe` for all users' processes |
| `recent-logins` | Recommended | Reads `/var/log/auth.log` and `/var/log/btmp` |
| `fsck-report` | Yes (for SMART) | `smartctl` requires root access |
| All other debug commands | No | Read-only `/proc` and log file access |

---

## License

MIT — use freely, fork, extend, contribute back.
