# qemu-vmnet

Native macOS networking for QEMU using `vmnet.framework` and `socket` networking.

## Requirements

- macOS 10.10 or later.
- Any QEMU version that supports `socket` networking), I tested it with 6.1.0 on ARM.

## Getting started

### Install qemu-vmnet

The only way for now is to have a working `Go` environment and build `qemu-vmnet` yourself.

### Start qemu-vmnet

You have to start `qemu-vmnet` with `sudo`, this is a requirement of `vmnet`.

Example:

```shell
sudo qemu-vmnet
```

### Add a network device

You need add a new network device to your virtual machine.

Note: `netdev` value is the `id` of the network device, can be any value.

Example:

```
-device virtio-net,netdev=net0
```

### Configure the network device

The network device you just added must be configured to use `socket` networking and UDP port `1234` (can be changed in the future, see [Options](#Options)).

Note: `localaddr` can be any free port, but you must specify it.

Example:

```
-netdev socket,id=net0,udp=:1234,localaddr=:1235
```

### Enjoy

Enjoy your fully working networking with a dedicated IP!

## Options

No options for now, the UDP listener is announced on port `1234` and `vmnet` is configured in NAT mode.

In the future the UDP port can be changed and maybe `vmnet` can be configure in bridged mode :)
