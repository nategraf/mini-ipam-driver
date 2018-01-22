package allocator

import (
    "fmt"
    "net"
    "github.com/nategraf/mini-ipam-driver/bytop"
)

// Allocator is simplified interface for managing IP addresses.
type Allocator interface {
    addrSpace() string

    AddPool(*net.IPNet) error
    RequestPool(int) (*net.IPNet, error)
    ReleasePool(*net.IPNet) error
    RequestAddress(*net.IPNet, net.IP) (net.IP, error)
    ReleaseAddress(net.IP) error
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
func (a *LocalAllocator) AddPool(pool *net.IPNet) error {
    masklen, addrlen := pool.Mask.Size()

    if addrlen != 32 {
        // This is not a proper IPv4 subnet. Abort!
        return fmt.Errorf("Only 32-bit IPv4 subnets can be added")
    }

    a.pools[masklen] = append(a.pools[masklen], pool)
    return nil
}

// RequestPool allocates a pool of the requested size.
// nil is returned if the request cannnot be fulfiled.
func (a *LocalAllocator) RequestPool(masklen int) (*net.IPNet, error) {
    var pool *net.IPNet
    var i int

    if masklen < 0 || masklen > 31 {
        return nil, fmt.Errorf("Masklen must be in the interval [0, 31]")
    }

    // Search up the pool lists for a large enough pool
    for i = masklen; i >= 0; i-- {
        s := a.pools[i]
        if len(s) > 0 {
            // Pop head
            pool, a.pools[i] = s[0], s[1:]
            break
        }
    }

    // If we didn't find a large enough pool return nil
    if pool == nil {
        return nil, fmt.Errorf("No pool availible to allocate a /%d subnet", masklen)
    }

    // Split the pool until we have the correct size
    for ; i < masklen; i++ {
        var extrapool *net.IPNet
        pool, extrapool = splitPool(pool)
        a.pools[i+1] = append(a.pools[i+1], extrapool)
    }

    a.allocated[pool.String()] = true
    return pool, nil
}

func (a *LocalAllocator) ReleasePool(pool *net.IPNet) error {
    if a.allocated[pool.String()] {
        a.AddPool(pool)
        delete(a.allocated, pool.String())
        return nil
    } else {
        return fmt.Errorf("Pool was never allocated: %s", pool.String())
    }
}

func (a *LocalAllocator) RequestAddress(pool *net.IPNet, ip net.IP) (net.IP, error) {
    // Make sure we allocated this pool
    if !a.allocated[pool.String()] {
        return nil, fmt.Errorf("Pool was never allocated: %s, pool.String()")
    }

    // Is this a specific ip request or do we choose?
    if ip != nil {
        if pool.Contains(ip) && !a.allocated[ip.String()] {
            a.allocated[ip.String()] = true
            return ip, nil
        }

        return nil, fmt.Errorf("Cannot allocate %s from pool %s", ip.String(), pool.String())
    } else {
        ip = pool.IP.To4()
        if ip == nil {
            // Not a v4 address
            return nil, fmt.Errorf("Pool is not a valid IPv4 subet: %s", pool.String())
        }

        // Find the highest address in the pool (broadcast address)
        limit := bytop.Or(bytop.Not(pool.Mask, nil), ip, nil)
        bytop.Add(ip, 1, ip) // Add one to get past network address
        for ; !bytop.Equal(ip, limit); bytop.Add(ip, 1, ip) {
            if !a.allocated[ip.String()] {
                a.allocated[ip.String()] = true
                return ip, nil
            }
        }

        // Pool must be full
        return nil, fmt.Errorf("Pool is exhausted: %s", pool.String())
    }
}
func (a *LocalAllocator) ReleaseAddress(ip net.IP) error {
    ip = ip.To4()
    if ip == nil {
        return fmt.Errorf("Given IP address is not a valid IPv4 address: %s", ip.String())
    }

    if a.allocated[ip.String()] {
        delete(a.allocated, ip.String())
        return nil
    } else {
        return fmt.Errorf("IP address was never allocated: %s", ip.String())
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
