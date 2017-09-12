## corectl pull

Pulls a CoreOS image from upstream

### Synopsis


Pulls a CoreOS image from upstream

```
corectl pull [flags]
```

### Options

```
  -c, --channel string   CoreOS channel (default "alpha")
  -f, --force            forces the rebuild of an image, if already local
  -h, --help             help for pull
  -v, --version string   CoreOS version (default "latest")
  -w, --warmup           ensures that all (populated) channels are on their latest versions
```

### Options inherited from parent commands

```
  -d, --debug   adds additional verbosity, and options, directed at debugging purposes and power users
```

### SEE ALSO
* [corectl](corectl.md)	 - CoreOS over macOS made simple.

