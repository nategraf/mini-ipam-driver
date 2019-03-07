# Mini IPAM Driver
### A libnetwork ipam driver focused on allocating small subnets

Due to a lack of method for specifying the subnet size without explicitly setting the subnet in the default IPAM driver, I created this IPAM driver to allocate subnets of a specific size, but unspecified location.

Additionally I wrote this IPAM driver in the hopes that it could be a useful template for other developers interested in writing an IPAM driver. So if you are trying to trying to figure out how to write an IPAM driver, I hope this code, and my comments below are helpful!

This IPAM plugin is basic. It only handles requests over Unix socket, and stores state in a temp file. This may improve with time, but only as I need it as long as I am the only one using it. If you want to use this driver in your systems hop on [Discord](https://discord.gg/gH9ZgeT) and let me know you are using it or [send me an email](mailto:nategraf1@gmail.com)! I'd be happy to refine it for others to use!

This driver is written in support of my larger project [Naumachia](https://github.com/nategraf/Naumachia). Check it out!

## Usage
Right now installation is pretty much self-guided:
1. Download the binary from the Releases tab
2. Run `sudo ./driver`
3. Start using the driver! (e.g. `docker network create "foo" --ipam-driver mini`)

There is one driver option `com.github.mini.cidr_mask_length` which allows you to set the subnet mask length for the request subnet to an integer between 0 and 31 inclusive to control subet size.

You can create scripts around this to have it start on boot (e.g. with `upstart` or `cron @reboot`) to make things easier.

## Installation as a service with SysV (Debian/Ubuntu)
```bash
# Download the service script and install it to init.d
sudo curl -L https://raw.githubusercontent.com/nategraf/mini-ipam-driver/master/sysv.sh -o /etc/init.d/mini-ipam
sudo chmod +x /etc/init.d/mini-ipam

# Download the driver to usr/local/bin
sudo curl -L https://github.com/nategraf/mini-ipam-driver/releases/latest/download/mini-ipam-driver.linux.x64 -o /usr/local/bin/mini-ipam
sudo chmod +x /usr/local/bin/mini-ipam

# Activate the service
sudo update-rc.d mini-ipam defaults
sudo service mini-ipam start
```
or
```
curl -L http://bit.ly/10hA8iC | sudo bash
```

## How to write an IPAM driver
Any driver you will write for [libnetwork](https://github.com/docker/libnetwork) (Docker's networking stack) will be configured as a "[remote](https://github.com/docker/libnetwork/blob/master/docs/remote.md)" server which libnetwork will [RPC](https://en.wikipedia.org/wiki/Remote_procedure_call) to accomplish the some work. This server can be implemented in any language and be served either on a Unix socket locally or an HTTP server somewhere else.

For IPAM, the API we are implementing is specified in [ipam.md](https://github.com/docker/libnetwork/blob/master/docs/ipam.md)

Although this can be done in any language or framework, libnetwork lends a helping hand for Golang developers with a [basic framework for plugins](https://github.com/docker/go-plugins-helpers) including the [ipam plugin](https://github.com/docker/go-plugins-helpers/tree/master/ipam)

The implementation in this repo uses the [provided ipam helper code](https://github.com/docker/go-plugins-helpers/tree/master/ipam) and additionally defines it's own further simplified interface for an `Allocator` to separate the logic of the driver interaction from the nitty gritty of allocation. This is done to facilitate the creation of a suitable global IPAM allocator using an external store in the future, as well as improve readability. Hopefully you can benefit from this and use some or all of the driver code for your implementation.

The actual allocator logic itself is in `allocator.go`. The approach I use is a inspired by the ["buddy system" for memory allocation](https://en.wikipedia.org/wiki/Buddy_memory_allocation). The tracking strcuture is a [32 level list](https://github.com/nategraf/mini-ipam-driver/blob/master/allocator/allocator.go#L62), in which each level contains a list of availible subnets of that mask length (size). As pools are allocated the larger pools will be [broken up and populate down](https://github.com/nategraf/mini-ipam-driver/blob/master/allocator/allocator.go#L139-L143) the lists (from larger to smaller) and as pools are freed the pools will [coalesce and move back up](https://github.com/nategraf/mini-ipam-driver/blob/master/allocator/allocator.go#L95-L103) the lists (from smaller to larger). Additionally there is a [map of allocated pools and addresses](https://github.com/nategraf/mini-ipam-driver/blob/master/allocator/allocator.go#L63) in string form to make querying for allocated resources fast.

For storage I employ a simple strategy of saving to a file on each update and loading form that file on startup. An [asynchronous goroutine](https://github.com/nategraf/mini-ipam-driver/blob/master/allocator/allocator.go#L68) is responsible for saving the current state, and receives [notifications via condition variable](https://github.com/nategraf/mini-ipam-driver/blob/master/allocator/allocator.go#L261-L265) when it's time to work.

I hope this implementation is a helpful starting point for your own IPAM module!
