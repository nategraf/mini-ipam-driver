package allocator

import (
    "net"
    "github.com/nategraf/mini-ipam-driver/bytop"
)

// Allocator is simplified interface for managing IP addresses.
type Allocator interface {
    addrSpace() string

    AddPool(*net.IPNet)
    RequestPool(int) *net.IPNet
    ReleasePool(*net.IPNet)
    RequestAddress(*net.IPNet, net.IP) net.IP
    ReleaseAddress(net.IP)
}

const NilAS = "null"

func AddrSpace(a Allocator) string {
    if a == nil {
        return NilAS
    }
    return a.addrSpace()
}

// LocalAllocator is an allocator which stores data in process memory.
// It does not use an external data store and therefore cannot be used across a cluster.
type LocalAllocator struct{
    pools [][]*net.IPNet
    allocated map[string]bool
}

// NewLocalAllocator creates and initializes a new LocalAllocator
func NewLocalAllocator() *LocalAllocator {
    return &LocalAllocator{
        pools: make([][]*net.IPNet, 32),
        allocated: make(map[string]bool),
    }
}

func (a *LocalAllocator) addrSpace() string {
    return "local"
}

// AddPool adds a new subnet to be used in allocations.
func (a *LocalAllocator) AddPool(pool *net.IPNet) {
    masklen, addrlen := pool.Mask.Size()

    if addrlen != 32 {
        // This is not a proper IPv4 subnet. Abort!
        return
    }

    a.pools[masklen] = append(a.pools[masklen], pool)
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
        a.pools[i+1] = append(a.pools[i+1], extrapool)
    }

    a.allocated[pool.String()] = true
    return pool
}

func (a *LocalAllocator) ReleasePool(pool *net.IPNet) {
    if a.allocated[pool.String()] {
        a.AddPool(pool)
        delete(a.allocated, pool.String())
    }
}

func (a *LocalAllocator) RequestAddress(pool *net.IPNet, ip net.IP) net.IP {
    // Make sure we allocated this pool
    if !a.allocated[pool.String()] {
        return nil
    }

    // Is this a specific ip request or do we choose?
    if ip != nil {
        if pool.Contains(ip) && !a.allocated[ip.String()] {
            a.allocated[ip.String()] = true
            return ip
        }

        return nil
    } else {
        ip = pool.IP.To4()
        if ip == nil {
            // Not a v4 address
            return nil
        }

        // Find the highest address in the pool (broadcast address)
        limit := bytop.Or(bytop.Not(pool.Mask, nil), ip, nil)
        bytop.Add(ip, 1, ip) // Add one to get past network address
        for ; !bytop.Equal(ip, limit); bytop.Add(ip, 1, ip) {
            if !a.allocated[ip.String()] {
                a.allocated[ip.String()] = true
                return ip
            }
        }

        // Pool must be full
        return nil
    }
}
func (a *LocalAllocator) ReleaseAddress(ip net.IP) {
    ip = ip.To4()
    if ip != nil && a.allocated[ip.String()] {
        delete(a.allocated, ip.String())
    }
}

func copyIPNet(ipnet *net.IPNet) *net.IPNet {
    ip := make(net.IP, len(ipnet.IP))
    mask := make(net.IPMask, len(ipnet.Mask))
    copy(ip, ipnet.IP)
    copy(mask, ipnet.Mask)
    return &net.IPNet{ IP: ip, Mask: mask }
}

func splitPool(pool *net.IPNet) (*net.IPNet, *net.IPNet) {
    masklen, addrlen := pool.Mask.Size()

    if addrlen != 32 || masklen >= 32 {
        return nil, nil
    }

    left := copyIPNet(pool)
    right := copyIPNet(pool)

    bytop.FlipBit(masklen, right.IP)   //Flip the bit to create a new network
    bytop.FlipBit(masklen, right.Mask) // Lengthen the mask by 1
    bytop.FlipBit(masklen, left.Mask)  // Lengthen the mask by 1

    return left, right
}
