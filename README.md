# CoreOS _over_ macOS made simple

| read this first |
| :----------- |
|  - You **must** be running macOS Yosemite, 10.10.3, or later on a 2010, or later, Macintosh (i.e. one with a CPU that supports EPT) for everything to work. |
|  - Starting with **0.7.18** the bundled `qcow-tool` helper has a _runtime_ dependency in `libev`. Until we sort out how to build it statically, you need to make it available in the local system - if you are using [homebrew](http://brew.sh) that is as easy as `brew install libev`.|
|  - If you are still using **any** version of VirtualBox older than 4.3.30 then `corectl` will **crash** your system either if VirtualBox is running, or had been run previously after the last reboot (see **xhyve**'s issues [#5](https://github.com/mist64/xhyve/issues/5) and [#9](https://github.com/mist64/xhyve/issues/9) for the full context). So, if for some reason, you are unable to update VirtualBox to the latest, either of the 4.x or 5.x streams, and were using it in your current session please make sure to restart your Mac before attempting to run `corectl`. |
|  - If you are using some sort of desktop firewall in your macOS host (ESET, Little Snitch, whatever) please make sure that it **allows traffic from/to the `bridge100` interface to the host** as otherwise no VM will ever able to succefully boot (as it can't fetch the ignition configs, etc from the host's running `corectld`)|


# step by step instructions

## install **corectl**

### installing a release build (prefered for end users)

#### via [homebrew](http://brew.sh)

```
❯❯❯ brew install corectl
```

#### downloading from GitHub

just go to our **[releases](https://github.com/genevera/corectl/releases)**
page and download the tarball with the binaries to your system, and then
unpack its' contents placing them somewhere in some directory in your
`${PATH}` (`/usr/local/bin/` is usually a good choice)

### build it locally (for power users)

  ```
  ❯❯❯ mkdir -p ${GOPATH}/src/github.com/genevera/
  ❯❯❯ cd ${GOPATH}/src/github.com/genevera/
  ❯❯❯ git clone git@github.com:genevera/corectl.git
  ❯❯❯ cd corectl
  ❯❯❯ make
  ```

  > the built binaries will _then_ appear inside
  > `${GOPATH}/src/github.com/genevera/corectl/bin`

## **start the** corectl **server daemon** (**corectld**)
> this is a **required** step starting with **corectl**'s **0.7.0** release

  ```
  ❯❯❯ /usr/local/bin/corectld
  ```

## kickstart a CoreOS VM
> the following command will fetch the `latest` CoreOS Alpha image
> available, if not already available locally, verify its integrity, and then
>boot it.

  ```
  ❯❯❯ corectl run
  ```

In your terminal you will shortly see something like the following...

  ```
  ❯❯❯  corectl run
  ---> 'B4AF19D1-DDEE-4A16-8058-1A7C3579F203' started successfully with address 192.168.64.210 and PID 76202
  ---> 'B4AF19D1-DDEE-4A16-8058-1A7C3579F203' boot logs can be found at '/Users/am/.coreos/running/B4AF19D1-DDEE-4A16-8058-1A7C3579F203/log'
  ---> 'B4AF19D1-DDEE-4A16-8058-1A7C3579F203' console can be found at '/Users/am/.coreos/running/B4AF19D1-DDEE-4A16-8058-1A7C3579F203/tty'
```

Accessing the newly created CoreOS instance is just a few more clicks away...
  ```
  ❯❯❯  corectl ssh B4AF19D1-DDEE-4A16-8058-1A7C3579F203
  ```

## usage _(straight from the online help)_
### **corectld**
  ```
  CoreOS over macOS made simple. <http://github.com/genevera/corectl>
  Copyright (c) 2015-2016, António Meireles

  Usage:
      corectld [flags]
      corectld [command]

  Available Commands:
      start       Starts corectld
      status      Shows corectld status
      stop        Stops corectld
      version     Shows version information

  Flags:
    -d, --debug   adds additional verbosity, and options, directed at debugging purposes and power users

  Use "corectld [command] --help" for more information about a command.

  All flags can also be set via upper-case environment variables prefixed with "COREOS_"
  For example, "--debug" => "COREOS_DEBUG"
  ```
### **corectl**
  ```
  CoreOS over macOS made simple. <http://github.com/genevera/corectl>
  Copyright (c) 2015-2016, António Meireles

  Usage:
      corectl [flags]
      corectl [command]

  Available Commands:
      kill        Halts one or more running CoreOS instances
      load        Loads CoreOS instances defined in an instrumentation file.
      ls          Lists the CoreOS images available locally
      ps          Lists running CoreOS instances
      pull        Pulls a CoreOS image from upstream
      put         copy file to inside VM
      query       Display information about the running CoreOS instances
      rm          Remove(s) CoreOS image(s) from the local filesystem
      run         Boots a new CoreOS instance
      ssh         Attach to or run commands inside a running CoreOS instance
      version     Shows version information

  Flags:
    -d, --debug   adds additional verbosity, and options, directed at debugging purposes and power users

  Use "corectl [command] --help" for more information about a command.

  All flags can also be set via upper-case environment variables prefixed with "COREOS_"
  For example, "--debug" => "COREOS_DEBUG"
  ```

  > [here](documentation/markdown/corectl.md) you can find the full
  > auto-generated documentation.

## simple usage recipe: a **docker** and **rkt** playground

### create a volume to store your persistent data
  > [`qcow-tool`](https://github.com/mirage/ocaml-qcow), that we use below, is
  > shipped together with **corectl** and creates qcow2 images.
  >
  > Please do note that the `--size` argument
  > **must** to be suffixed the _right_ way - `KiB`/`MiB`/`GiB`/`TiB`/`PiB`

  ```
  ❯❯❯ qcow-tool create --size=16GiB var_lib_docker.img.qcow2
  ```
  > will become `/var/lib/{docker|rkt}`. In this example case we created a
  > **QCow2** volume with 16GB.

| a quick note regarding Raw volumes |
| :--------- |
| **Raw** volumes were the default until version **[0.7.12](https://github.com/genevera/corectl/releases/tag/v0.7.12)**. <br> They are still supported but become a deprecated feature that may disappear some point in the future. |

### *format* and label it
  > we'll format and label the newly created volume from within a transient VM
  > as it's the simplest way. We're formatting it with `ext4` but you can choose
  > any filesystem you like assuming it is a CoreOS supported one.

  ```
  ❯❯❯ corectl run  --name foo --volume=var_lib_docker.img.qcow2
  ❯❯❯ corectl ssh foo "sudo mke2fs -b 1024 -i 1024 -t ext4 -m0 /dev/vda && \
        sudo e2label /dev/vda rkthdd "
  ❯❯❯ corectl halt foo
  ```

  above, we labeled our volume `rkthdd` which is the *signature* that our
  [*recipe*](examples/cloud-init/docker-only-with-persistent-storage.txt) expects.

  >by relying in *labels* for volume identification we get around the issues we'd
  >have otherwise if we were depending on the actual volume name (/dev/vd...) as
  >those would have to be hardcoded (an issue, if one is mix and matching
  >multiple recipes all dealing with different volumes...)

### start your **docker** and **rkt** playground.
  ```
  ❯❯❯ UUID=deadbeef-dead-dead-dead-deaddeafbeef \
    corectl run --volume absolute_or_relative_path/to/persistent.img \
    --cloud_config examples/cloud-init/docker-only-with-persistent-storage.txt \
    --cpus 2 --memory 2048 --name containerland
  ```
 this will start a VM named `containerland` with the
 volume we created previously attached, 2 virtual cores and 2GB of RAM. The
 provided [cloud-config](examples/cloud-init/docker-only-with-persistent-storage.txt)
 will format the given volume (if it wasn't yet) and bind mount both
 `/var/lib/rkt` and `/var/lib/docker` on top of it. Docker will also become
 available through socket activation.

 > above we passed arguments to the VM both via environment variables and
 > command flags. both ways are fully supported, just use whatever suits your
 > needs better.

### now...

  ```
  ❯❯❯ corectl ps
  Server:
    Version:      0.7.0
    Go Version:   go1.6.2
    Built:        Mon Jul 04 10:05:51 WEST 2016
    OS/Arch:      darwin/amd64

    Pid:          76155
    Uptime:       37 minutes ago

  Activity:
  Active VMs:     1
  Total Memory:   2048
  Total vCores:   2

  UUID:           A163767A-78DC-41F9-AA66-E57B6C6CAB1A
    Name:         containerland
    Version:      1097.0.0
    Channel:      alpha
    vCPUs:        3
    Memory (MB):  2048
    Pid:          76807
    Uptime:       25 minutes ago
    Sees World:   true
    cloud-config: /Users/am/code/corectl/src/github.com/genevera/corectl/examples/cloud-init/docker-only-with-persistent-storage.txt
    Network:
      eth0:       192.168.64.2
    Volumes:
    /dev/vda      /Users/am/code/corectl/persistentData/var_lib_docker.img
  ```

  ```
  ❯❯❯ docker -H $(corectl q containerland --ip):2375 images -a
  REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
  centos              latest              05188b417f30        2 days ago          196.8 MB
  busybox             latest              2b8fd9751c4c        10 days ago         1.093 MB
  fedora              latest              f9873d530588        13 days ago         204.4 MB
  ```

or ...

  ```
    ❯❯❯ corectl ssh containerland
    CoreOS stable (1097.0.0)
    Last login: Mon Jul  4 09:17:26 2016 from 192.168.64.1
    Update Strategy: No Reboots
  ```
or ...
  ```
  ❯❯❯ corectl ssh containerland "sudo rkt list"
  UUID	APP	IMAGE NAME	STATE	CREATED	STARTED	NETWORKS
  ```

> All running VMs become reachable by name transparently on the host using
>  macOS' native name resolution machinery
>  ```
>  ❯❯❯ ping -c 1 containerland
>  PING containerland.coreos.local (192.168.64.2): 56 data bytes
>  64 bytes from 192.168.64.2: icmp_seq=0 ttl=64 time=0.239 ms
>
>  --- containerland.coreos.local ping statistics ---
>  1 packets transmitted, 1 packets received, 0.0% packet loss
>  round-trip min/avg/max/stddev = 0.239/0.239/0.239/0.000 ms
>  ```

### have fun!

## Tracing

Thanks to [hyperkit](https://github.com/docker/hyperkit) (that we consume as
`corectld.runner`) there are available a  number of static DTrace probes to
simplify investigation of performance problems. To list the probes supported by
your version of corectl, type the following command while `corectld` is running:

 `$ sudo dtrace -l -P 'hyperkit$target' -p $(pgrep corectld.runner)`

Refer to scripts in `examples/dtrace/` directory for examples of possible usage
and available probes.

# projects using **corectl**

- [Rimas Mocevicius](https://github.com/rimusz) entire toolset of macOS GUI apps
is now using **corectl** underneath, and has become part of the
[genevera](http://github.com/genevera) project
  - **[Corectl.app controlling app of corectld server daemon](https://github.com/genevera/corectl.app)**
  - **[CoreOS VM for macOS](https://github.com/genevera/coreos-osx)**
  - **[Kubernetes Solo Cluster for macOS](https://github.com/genevera/kube-solo-osx)**
  - **[Multi node Kubernetes Cluster for macOS](https://github.com/genevera/kube-cluster-osx)**

# acknowledgements

-  [Michael Steil](https://github.com/mist64) for releasing into the wild his
   awesome [xhyve](https://github.com/mist64/xhyve) lightweight macOS
   virtualization solution
-  [Docker Inc](http://www.docker.com/) for keep improving it through
   [hyperkit](https://github.com/docker/hyperkit).
-  [Brandon Philips](https://github.com/philips), from
   [CoreOS](http://www.coreos.com), who come with the original, **bash** based,
   [coreos-xhyve](https://github.com/coreos/coreos-xhyve) prototype that this
   project supersedes

# contributing

**corectl** is an [open source](http://opensource.org/osd) project released under
the [Apache License, Version 2.0](http://opensource.org/licenses/Apache-2.0),
contributions and sugestions are gladly welcomed!
