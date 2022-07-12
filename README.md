# qemu-vmnet

Native macOS networking for QEMU using `vmnet.framework` and `socket` networking.

## Requirements

- macOS 10.10 or later.
- Any QEMU version that supports `socket` networking, I tested it with 6.1.0 on ARM.

## Getting started

### Install qemu-vmnet

The only way for now is to have a working `Go` environment and install `qemu-vmnet` with:

```shell
go install github.com/adnsio/qemu-vmnet@latest
```

### Start qemu-vmnet

You have to start `qemu-vmnet` with `sudo`, this is a requirement of `vmnet`.

Example:

```shell
sudo qemu-vmnet
```

### Add a network device

You need to add a new network device to your virtual machine.

Note: `netdev` value is the `id` of the network device, can be any value.

Example:

```
-device virtio-net,netdev=net0
```

### Configure the network device

The network device you just added must be configured to use `socket` networking and UDP port `2233` (can be changed, see [Options](#Options)).

Note: `localaddr` can be any free port, but you must specify it.

Example:

```
-netdev socket,id=net0,udp=:2233,localaddr=:1122
```

### Enjoy

Enjoy your fully working networking with a dedicated IP!

## Options

- `-address` sets the listening address (default ":2233")
- `-cpuprofile file` write cpu profile to file
- `-debug` sets log level to debug
- `-memprofile file` write memory profile to file
- `-trace` sets log level to trace

In the future `vmnet` can be configured in bridged mode :)
