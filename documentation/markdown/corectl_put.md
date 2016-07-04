## corectl put

copy file to inside VM

### Synopsis


copy file to inside VM

```
corectl put path/to/file VMid:/file/path/on/destination
```

### Examples

```
  // copies 'filePath' into '/destinationPath' inside VMid
  corectl put filePath VMid:/destinationPath
```

### Options inherited from parent commands

```
  -d, --debug   adds additional verbosity, and options, directed at debugging purposes and power users
```

### SEE ALSO
* [corectl](corectl.md)	 - CoreOS over macOS made simple.

