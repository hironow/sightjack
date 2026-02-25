## sightjack init

Create .siren/config.yaml interactively

### Synopsis

Initialize a new sightjack project by creating .siren/config.yaml.

Interactively prompts for Linear team, project, language, and
strictness level. Also creates .gitignore, installs d-mail skills,
and sets up mail directories. Run 'sightjack doctor' after init
to verify the environment.

```
sightjack init [path] [flags]
```

### Examples

```
  # Initialize in current directory
  sightjack init

  # Initialize in a specific directory
  sightjack init /path/to/project

  # After init, verify environment
  sightjack init && sightjack doctor
```

### Options

```
  -h, --help   help for init
```

### Options inherited from parent commands

```
  -c, --config string   Config file path (default ".siren/config.yaml")
  -n, --dry-run         Generate prompts without executing Claude
  -l, --lang string     Language override (ja/en)
  -v, --verbose         Verbose logging
```

### SEE ALSO

* [sightjack](sightjack.md)  - SIREN-inspired issue architecture tool for Linear
