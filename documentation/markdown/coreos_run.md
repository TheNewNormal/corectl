## coreos run

starts a new CoreOS VM

### Synopsis


starts a new CoreOS VM

```
coreos run
```

### Options

```
      --cdrom="": append an CDROM (.iso) to VM
      --channel="alpha": CoreOS channel
      --cloud_config="": cloud-config file location (either URL or local path)
      --cpus=1: VM's vCPUS
  -d, --detached[=false]: starts the VM in detached (background) mode
      --extra="": additional arguments to xhyve hypervisor
  -h, --help[=false]: help for run
      --memory=1024: VM's RAM
  -n, --name="": names the VM. (the default is the uuid)
      --net=[]: append additional network interfaces to VM
      --root="": append a (persistent) root volume to VM
      --sshkey="": VM's default ssh key
      --uuid="random": VM's UUID
      --version="latest": CoreOS version
      --volume=[]: append disk volumes to VM
      --xhyve="/usr/local/bin/xhyve": xhyve binary to use
```

### Options inherited from parent commands

```
      --debug[=false]: adds extra verbosity, and options, for debugging purposes and/or power users
      --json[=false]: outputs in JSON for easy 3rd party integration
```

### SEE ALSO
* [coreos](coreos.md)	 - CoreOS, on top of OS X and xhyve, made simple.

