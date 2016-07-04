## corectl query

Display information about the running CoreOS instances

### Synopsis


Display information about the running CoreOS instances

```
corectl query [VMids]
```

### Options

```
  -a, --all      display a table with extended information about running CoreOS instances
  -i, --ip       displays given instance IP address
  -j, --json     outputs in JSON for easy 3rd party integration
  -l, --log      displays given instance boot logs location
  -o, --online   tells if at boot time VM had connectivity to outter world
  -t, --tty      displays given instance tty's location
  -u, --up       tells if a given VM is up or not
  -U, --uuid     returns VM's UUID
```

### Options inherited from parent commands

```
  -d, --debug   adds additional verbosity, and options, directed at debugging purposes and power users
```

### SEE ALSO
* [corectl](corectl.md)	 - CoreOS over macOS made simple.

