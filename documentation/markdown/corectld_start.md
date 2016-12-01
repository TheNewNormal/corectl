## corectld start

Starts corectld

### Synopsis


Starts corectld

```
corectld start
```

### Options

```
      --dns-port string                     embedded dns server port (default "15353")
  -D, --domain string                       sets the dns domain under which the created VMs will operate (default "coreos.local")
  -r, --recursive-nameservers stringSlice   coma separated list of the recursive nameservers to be used by the embedded dns server (default [8.8.8.8:53,8.8.4.4:53])
  -u, --user string                         sets the user that will 'own' the corectld instance
```

### Options inherited from parent commands

```
  -d, --debug   adds additional verbosity, and options, directed at debugging purposes and power users
```

### SEE ALSO
* [corectld](corectld.md)	 - CoreOS over macOS made simple.

