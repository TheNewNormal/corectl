## corectl run

Starts a new CoreOS instance

### Synopsis


Starts a new CoreOS instance

```
corectl run
```

### Options

```
      --cdrom string          append an CDROM (.iso) to VM
      --channel string        CoreOS channel (default "alpha")
      --cloud_config string   cloud-config file location (either a remote URL or a local path)
      --cpus int              VM's vCPUS (default 1)
  -d, --detached              starts the VM in detached (background) mode
  -l, --local latest          consumes whatever image is latest locally instead of looking online unless there's nothing available.
      --memory int            VM's RAM, in MB, per instance (1024 < memory < 8192) (default 1024)
  -n, --name string           names the VM. (if absent defaults to VM's UUID)
      --root string           append a (persistent) root volume to VM
      --sshkey string         VM's default ssh key
      --tap string            append tap interface to VM
      --uuid string           VM's UUID (default "random")
      --version string        CoreOS version (default "latest")
      --volume value          append disk volumes to VM (default [])
```

### Options inherited from parent commands

```
      --debug   adds extra verbosity, and options, for debugging purposes and/or power users
```

### SEE ALSO
* [corectl](corectl.md)	 - CoreOS over OSX made simple.

