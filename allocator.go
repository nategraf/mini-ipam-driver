package allocator

import (
    "fmt"
    "net"
)

// Allocator is simplified interface for managing IP addresses.
type Allocator interface {
    AddPool(net.IPNet)
    RequestPool(int) net.IPNet
    ReleasePool(net.IPNet)
    RequestAddress(net.IPNet) net.IP
    ReleaseAddress(net.IPNet, net.IP)
}

// LocalAllocator is an allocator which stores data in process memory.
// It does not use an external data store and therefore cannot be used across a cluster.
type LocalAllocator struct{
    pools [][]net.IP
    addrs map[net.IPNet]map[net.IP]bool
}

// NewLocalAllocator creates and initializes a new LocalAllocator
func NewLocalAllocator() *LocalAllocator {
    return &LocalAllocator{
        pools: make([][]net.IP, 32),
        addrs: make(map[net.IPNet]map[net.IP]bool)
    }
}

// AddPool adds a new subnet to be used in allocations.
func (a *LocalAllocator) AddPool(pool net.IPNet) {
    masklen, addrlen = pool.Mask.Size()

    if addrlen != 32 {
        // This is not a proper IPv4 subnet. Abort!
        return
    }

    s := a.pools[masklen]
    if s == nil {
        s = make([]net.IP)
        a.pools[masklen] = s
    }

    append(s, pool)
}

// RequestPool allocates a pool of the requested size.
// nil is returned if the request cannnot be fulfiled.
func (a *LocalAllocator) RequestPool(masklen int) net.IPNet {
    var pool net.IPNet
    var i int

    // Search up the pool lists for a large enough pool
    for i = masklen; i >= 0; i-- {
        s := a.pools[i]
        if s == nil || len(s) == 0 {
            continue
        }

        // Pop head
        pool, a.pools[i] = s[0], s[1:]
    }

    // If we didn't find a large enough pool return nil
    if pool == nil {
        return nil
    }

    // Split the pool until we have the correct size
    for ; i < masklen; i++ {
    }
}
