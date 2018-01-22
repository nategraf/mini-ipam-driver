# Mini IPAM Driver
### A libnetwork ipam driver focused on allocating small subnets

Due to a lack of method for specifying the subnet size without explicitly setting the subnet in the default IPAM driver, I created this IPAM driver to allocate subnets of a specific size, but unspecified location.

Additionally I wrote this IPAM driver in the hopes that it could be a useful template for other developers interested in writing an IPAM driver. So if you are trying to trying to figure out how to write an IPAM driver, I hope this code, and my comments below are helpful!

In its current form this IPAM plugin is very basic. It only handles requests over Unix socket. It stores allocations in memory, and is not configurable. This may improve with time, but only as I need it as long as I am the only one using it. If you want to use this driver in your systems hop on [Discord](https://discord.gg/gH9ZgeT) and let me know you are using it or [send me an email](mailto:nategraf1@gmail.com)! I'd be happy to refine it for others to use!

This driver is written in support of my larger project [Naumachia](https://github.com/nategraf/Naumachia). Check it out!

## Usage
Right now installation is pretty much self-guided:
1. Clone this repo to your server
2. Navigate to the `/driver` directory and build it with `go build`
3. Run `sudo ./driver`
4. Start using the driver (e.g. `docker network create "foo" --ipam-driver mini`)

Obviously you can create scripts around this to have it start on boot (e.g. with `upstart` or `cron @reboot`) to make things easier. I currently don't have a portable solution in this repo

## How to write an IPAM driver
Any driver you will write for [libnetwork](https://github.com/docker/libnetwork) (Docker's networking stack) will be configured as a "[remote](https://github.com/docker/libnetwork/blob/master/docs/remote.md)" server which libnetwork will [RPC](https://en.wikipedia.org/wiki/Remote_procedure_call) to accomplish the some work. This server can be implemented in any language and be served either on a Unix socket locally or an HTTP server somewhere else.

For IPAM, the API we are implementing is specified in [ipam.md](https://github.com/docker/libnetwork/blob/master/docs/ipam.md)

Although this can be done in any language or framework, libnetwork lends a helping hand for Golang developers with a [basic framework for plugins](https://github.com/docker/go-plugins-helpers) including the [ipam plugin](https://github.com/docker/go-plugins-helpers/tree/master/ipam)

The implementation in this repo uses the [provided ipam helper code](https://github.com/docker/go-plugins-helpers/tree/master/ipam) and additionally defines it's own further simplified interface for an `Allocator` to separate the logic of the driver interaction from the nitty gritty of allocation. This is done to facilitate the creation of a suitable global IPAM allocator using an external store in the future, as well as improve readability. Hopefully you can benefit from this and use some or all of the driver code for your implementation.
