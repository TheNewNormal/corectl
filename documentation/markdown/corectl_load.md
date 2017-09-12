## corectl load

Loads CoreOS instances defined in an instrumentation file.

### Synopsis


Loads CoreOS instances defined in an instrumentation file (either in TOML, JSON or YAML format).
VMs are always launched by alphabetical order relative to their names.

```
corectl load path/to/yourProfile [flags]
```

### Examples

```
  corectl load profiles/demo.toml
```

### Options

```
  -h, --help   help for load
```

### Options inherited from parent commands

```
  -d, --debug   adds additional verbosity, and options, directed at debugging purposes and power users
```

### SEE ALSO
* [corectl](corectl.md)	 - CoreOS over macOS made simple.

