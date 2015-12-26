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
      --debug   adds extra verbosity, and options, for debugging purposes and/or power users
```

### SEE ALSO
* [corectl](corectl.md)	 - CoreOS over OSX made simple.

