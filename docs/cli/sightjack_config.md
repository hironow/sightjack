## sightjack config

View or update sightjack configuration

### Synopsis

View or update the .siren/config.yaml configuration file.

### Examples

```
  sightjack config show
  sightjack config set tracker.team MY
  sightjack config set lang en
```

### Options

```
  -h, --help   help for config
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
      --no-color        Disable colored output (respects NO_COLOR env)
  -o, --output string   Output format: text, json (default "text")
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)	 - SIREN-inspired issue architecture tool for Linear
* [sightjack config set](sightjack_config_set.md)	 - Update a configuration value
* [sightjack config show](sightjack_config_show.md)	 - Display effective configuration

