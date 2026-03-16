## sightjack config set

Update a configuration value

### Synopsis

Update a configuration value in .siren/config.yaml.

Supported keys:
  tracker.team              Linear team key (e.g. MY)
  tracker.project           Linear project name
  tracker.cycle             Linear cycle name
  lang                      Language (ja or en)
  strictness.default        Default strictness level (fog, alert, lockdown)
  scan.chunk_size           Issues per scan chunk
  scan.max_concurrency      Max concurrent scan workers
  claude_cmd                Claude CLI command name (alias: assistant.command)
  model                     Claude model name (alias: assistant.model)
  timeout_sec               Claude timeout in seconds (alias: assistant.timeout_sec)
  scribe.enabled            Enable Scribe Agent (true/false)
  scribe.auto_discuss_rounds  Auto-discuss rounds (non-negative integer)
  retry.max_attempts        Max retry attempts (positive integer)
  retry.base_delay_sec      Base retry delay in seconds (positive integer)
  gate.auto_approve         Auto-approve convergence gate (true/false)
  gate.notify_cmd           Gate notification command
  gate.approve_cmd          Gate approval command
  gate.review_cmd           Gate review command
  gate.review_budget        Max review cycles (non-negative integer)
  gate.wait_timeout         D-Mail waiting phase timeout (e.g. 30m, 1h)
  labels.enabled            Enable Linear label assignment (true/false)
  labels.prefix             Linear label prefix
  labels.ready_label        Linear ready label

```
sightjack config set <key> <value> [path] [flags]
```

### Examples

```
  sightjack config set tracker.team MY
  sightjack config set lang en
  sightjack config set strictness.default alert
```

### Options

```
  -h, --help   help for set
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -q, --quiet           Suppress all stderr output
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack config](sightjack_config.md)	 - View or update sightjack configuration

