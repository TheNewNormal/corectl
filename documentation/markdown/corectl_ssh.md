## corectl ssh

Attach to or run commands inside a running CoreOS instance

### Synopsis


Attach to or run commands inside a running CoreOS instance

```
corectl ssh VMid ["command1;..."]
```

### Examples

```
  corectl ssh VMid                 // logins into VMid
  corectl ssh VMid "some commands" // runs 'some commands' inside VMid and exits
```

### Options inherited from parent commands

```
  -d, --debug   adds additional verbosity, and options, directed at debugging purposes and power users
```

### SEE ALSO
* [corectl](corectl.md)	 - CoreOS over macOS made simple.

