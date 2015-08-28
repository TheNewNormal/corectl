## coreos run

runs a new CoreOS container

### Synopsis


runs a new CoreOS container

```
coreos run
```

### Options

```
      --cdrom="": append an CDROM (.iso) to VM
      --channel="alpha": CoreOS channel
      --cloud_config="": cloud-config file location (either URL or local path)
      --cpus="1": VM's vCPUS
      --extra="": additional arguments to xhyve hypervisor
  -h, --help[=false]: help for run
      --memory="1024": VM's RAM
      --net=[]: append additional network interfaces to VM
      --root="": append a (persistent) root volume to VM
      --sshkey="": VM's default ssh key
      --uuid="random": VM's UUID
      --version="latest": CoreOS version
      --volume=[]: append disk volumes to VM
      --xhyve="/usr/local/bin/xhyve": xhyve binary to use
```

### SEE ALSO
* [coreos](coreos.md)	 - CoreOS, on top of OS X and xhyve, made simple.

