# CoreOS _over_ macOS made simple

**read this first**
-----------
 > - You **must** be running macOS Yosemite, 10.10.3, or later on a 2010,
 >   or later, Macintosh (i.e. one with a CPU that supports EPT) for everything
 >   to work.
 > - if you are still using **any** version of VirtualBox older than
 >   4.3.30 then **corectl** will **crash** your system either if VirtualBox is
 >   running, or had been run previously after the last reboot (see **xhyve**'s
 >   issues [#5](https://github.com/mist64/xhyve/issues/5) and
 >   [#9](https://github.com/mist64/xhyve/issues/9) for the full context). So,
 >   if for some reason, you are unable to update VirtualBox to the latest,
 >   either of the 4.x or 5.x streams, and were using it in your current session
 >   please make sure to restart your Mac before attempting to run **corectl**.


# step by step instructions

## install **corectl**

### installing a release build (prefered for end users)

#### via [homebrew's](http://brew.sh)

```
❯❯❯ brew install corectl
```

#### downloading from GitHub

just go to our **[releases](https://github.com/TheNewNormal/corectl/releases)**
page and download the tarball with the binaries to your system, and then
unpack its' contents placing them somewhere in some directory in your
`${PATH}` (`/usr/local/bin/` is usually a good choice)

### build it locally (for power users)

  ```
  ❯❯❯ mkdir -p ${GOPATH}/src/github.com/TheNewNormal/
  ❯❯❯ cd ${GOPATH}/src/github.com/TheNewNormal/
  ❯❯❯ git clone git@github.com:TheNewNormal/corectl.git
  ❯❯❯ cd corectl
  ❯❯❯ make
  ```

  > the built binaries will _then_ appear inside
  > `${GOPATH}/src/github.com/TheNewNormal/corectl/bin`

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

In your terminal you will shortly after something along...

  ```
  ❯❯❯  corectl run                                                                                                                                                                                                     ⏎ v0.7.x ✭ ✚ ✱
  ---> 'B4AF19D1-DDEE-4A16-8058-1A7C3579F203' started successfuly with address 192.168.64.210 and PID 76202
  ---> 'B4AF19D1-DDEE-4A16-8058-1A7C3579F203' boot logs can be found at '/Users/am/.coreos/running/B4AF19D1-DDEE-4A16-8058-1A7C3579F203/log'
  ---> 'B4AF19D1-DDEE-4A16-8058-1A7C3579F203' console can be found at '/Users/am/.coreos/running/B4AF19D1-DDEE-4A16-8058-1A7C3579F203/tty'
```

Accessing the newly craeted CoreOS instance is just a few more clicks away...
  ```
  ❯❯❯  corectl ssh B4AF19D1-DDEE-4A16-8058-1A7C3579F203
  ```

## usage _(straight from the online help)_
### **corectld**
  ```
  CoreOS over macOS made simple. <http://github.com/TheNewNormal/corectl>
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
  CoreOS over macOS made simple. <http://github.com/TheNewNormal/corectl>
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

  ```
  ❯❯❯ dd if=/dev/zero of=var_lib_docker.img  bs=1G count=16
  ```
> will become  `/var/lib/{docker|rkt}`. in this example case we created a volume
> with 16GB.

### *format* it

  ```
  ❯❯❯ /usr/local/Cellar/e2fsprogs/1.42.12/sbin/mke2fs -b 1024 -i 1024 -t ext4 -m0 -F var_lib_docker.img
  ```
  > requires [homebrew's](http://brew.sh) e2fsprogs package installed.
  >
  > `❯❯❯ brew install e2fsprogs`

### *label* it

  ```
  ❯❯❯ /usr/local/Cellar/e2fsprogs/1.42.12/sbin/e2label var_lib_docker.img rkthdd
  ```
  here, we labeled our volume `rkthdd` which is the *signature* that our
  [*recipe*](cloud-init/docker-only-with-persistent-storage.txt) expects.

  >by relying in *labels* for volume identification we get around the issues we'd
  >have otherwise if we were depending on the actual volume name (/dev/vd...) as
  >those would have to be hardcoded (an issue, if one is mix and matching
  >multiple recipes all dealing with different volumes...)

### start your **docker** and **rkt** playground.
  ```
  ❯❯❯ UUID=deadbeef-dead-dead-dead-deaddeafbeef \
    corectl run --volume absolute_or_relative_path/to/persistent.img \
    --cloud_config cloud-init/docker-only-with-persistent-storage.txt \
    --cpus 2 --memory 2048 --name containerland
  ```
 this will start a VM named `containerland` with the
 volume we created previously attached, 2 virtual cores and 2GB of RAM. The
 provided [cloud-config](cloud-init/docker-only-with-persistent-storage.txt)
 will format the given volume (if it wasn't yet) and bind mount both
 ``/var/lib/rkt` and `/var/lib/docker` on top of it. docker will also become
 available through socket activation.

 > above we passed arguments to the VM both via environment variables and
 > command flags. both ways are fully supported, just use whatever suits your
 > needs better.

### now...

  ```
  ❯❯❯ corectl ps
  Server:
    Version:	0.7.0
    Go Version:	go1.6.2
    Built:		Mon Jul 04 10:05:51 WEST 2016
    OS/Arch:	darwin/amd64

    Pid:		76155
    Uptime:	37 minutes ago

  Activity:
  Active VMs:	1
  Total Memory:	2048
  Total vCores:	2

  UUID:		A163767A-78DC-41F9-AA66-E57B6C6CAB1A
    Name:		containerland
    Version:	1097.0.0
    Channel:	alpha
    vCPUs:	3
    Memory (MB):	2048
    Pid:		76807
    Uptime:	25 minutes ago
    Sees World:	true
    cloud-config:	/Users/am/code/corectl/src/github.com/TheNewNormal/corectl/examples/cloud-init/docker-only-with-persistent-storage.txt
    Network:
      eth0:	192.168.64.2
    Volumes:
    /dev/vda	/Users/am/code/corectl/persistentData/var_lib_docker.img
  ```

  ```
  ❯❯❯ docker -H 192.168.64.220:2375 images -a
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
### have fun!

# projects using **corectl**

- [Rimas Mocevicius](https://github.com/rimusz) entire toolset of macOS GUI apps
is now using **corectl** underneath, and has become part of the
[TheNewNormal](http://github.com/TheNewNormal) project
  - **[CoreOS VM for macOS](https://github.com/TheNewNormal/coreos-osx)**
  - **[Kubernetes Solo Cluster for macOS](https://github.com/TheNewNormal/kube-solo-osx)**
  - **[Multi node Kubernetes Cluster for macOS](https://github.com/TheNewNormal/kube-cluster-osx)**

# acknowledgements

-  [Michael Steil](https://github.com/mist64) for releasing in the wild his
   awesome [xhyve](https://github.com/mist64/xhyve) lightweight macOS
   virtualization solution
-  [Docker Inc](http://www.docker.com/) for keep improving it through
   [hyperkit](https://github.com/docker/hyperkit).
-  [Brandon Philips](https://github.com/philips), from
   [CoreOS](http://www.coreos.com), who come with the original, **bash** based,
   [coreos-xhyve](https://github.com/coreos/coreos-xhyve) prototype that this
   project supersedes

# contributing

**corectl** is an [open source](http://opensource.org/osd) project release under
the [Apache License, Version 2.0](http://opensource.org/licenses/Apache-2.0),
ence contributions and sugestions are gladly welcomed!
