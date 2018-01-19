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
    pools [][]*net.IPNet
    addrs map[net.IPNet]map[net.IP]bool
}

// NewLocalAllocator creates and initializes a new LocalAllocator
func NewLocalAllocator() *LocalAllocator {
    return &LocalAllocator{
        pools: make([][]*net.IPNet, 32),
        addrs: make(map[net.IPNet]map[net.IP]bool)
    }
}

// AddPool adds a new subnet to be used in allocations.
func (a *LocalAllocator) AddPool(pool *net.IPNet) {
    masklen, addrlen = pool.Mask.Size()

    if addrlen != 32 {
        // This is not a proper IPv4 subnet. Abort!
        return
    }

    s := a.pools[masklen]
    if s == nil {
        s = make([]*net.IPNet)
        a.pools[masklen] = s
    }

    append(s, pool)
}

// RequestPool allocates a pool of the requested size.
// nil is returned if the request cannnot be fulfiled.
func (a *LocalAllocator) RequestPool(masklen int) *net.IPNet {
    var pool *net.IPNet
    var i int

    if masklen < 0 || masklen > 31 {
        return nil
    }

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
        var extrapool *net.IPNet
        pool, extrapool = splitPool(pool)
        append(a.pools[i+1], extrapool)
    }

    return pool
}

/* Currently not needed
func max(x, y int) int {
    if x > y {
        return x
    }
    return y
}

func arrAnd(a, b []byte) []byte {
    res = make([]byte, max(len(a), len(b)))
    for i, _ := range res {
        res[i] = 0xFF
        if i < len(a) {
            res[i] &= a[i]
        }
        if i < len(b) {
            res[i] &= b[i]
        }
    }
    return res
}
*/

func copyIPNet(ipnet *net.IPNet) *net.IPNet {
    ip := make(net.IP, len(ipnet.IP))
    mask := make(net.IPMask, len(ipnet.Mask))
    copy(ip, ipnet.IP)
    copy(mask, ipnet.Mask)
    return &net.IPNet{ IP: ip, Mask: mask }
}

// Flips a bit at the index, which is from left to right (most signifcant to least)
func flipBit(index int, arr []byte) {
    i, j := index / 8, index % 8
    arr[i] ^= 1 << (7 - j)
}

func splitPool(pool *net.IPNet) *net.IPNet, *net.IPNet {
    masklen, addrlen := pool.Mask.Size()

    if addrlen != 32 || masklen >= 32 {
        return nil, nil
    }

    left := copyIPNet(pool)
    right := copyIPNet(pool)

    flipBit(right.IP, masklen)   //Flip the bit to create a new network
    flipBit(right.Mask, masklen) // Lengthen the mask by 1
    flipBit(left.Mask, masklen)  // Lengthen the mask by 1

    return left, right
}
