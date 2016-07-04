## corectl run

Boots a new CoreOS instance

### Synopsis


Boots a new CoreOS instance

```
corectl run
```

### Options

```
  -b, --boot string           additional arguments to the kernel boot
  -c, --channel string        CoreOS channel stream (default "alpha")
  -L, --cloud_config string   cloud-config file location (either an URL or a local path)
  -N, --cpus int              VM number of virtual CPUs (default 1)
  -x, --extra string          additional arguments to the hypervisor
  -m, --memory int            VM's RAM, in MB, per instance (1024 < memory < 8192) (default 1024)
  -n, --name string           names the VM (default is VM's UUID)
  -o, --offline               doesn't go online to check for newer images than the locally available ones unless there is none available.
  -r, --root string           append a (persistent) root volume to VM
  -H, --shared-homedir        mounts (via NFS) host's homedir inside VM
  -k, --sshkey string         VM's default ssh key
  -u, --uuid string           VM's UUID (default "random")
  -v, --version string        CoreOS version (default "latest")
  -p, --volume value          append disk volumes to VM (default [])
```

### Options inherited from parent commands

```
  -d, --debug   adds additional verbosity, and options, directed at debugging purposes and power users
```

### SEE ALSO
* [corectl](corectl.md)	 - CoreOS over macOS made simple.

