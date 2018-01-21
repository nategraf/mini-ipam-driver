package main

import (
    "fmt"
    "net"
    "regexp"
    "github.com/docker/libnetwork/types"
    "github.com/docker/go-plugins-helpers/ipam"
    alloc "github.com/nategraf/mini-ipam-driver/allocator"
)

var (
    defaultPools = parsePools([]string{"172.16.0.0/16"})
    poolIdRe = regexp.MustCompile("([a-zA-Z0-9_]+):([a-zA-Z0-9./]+)")
)

const (
    socketAddress = "/run/docker/plugins/mini.sock"
    defaultMasklen = 28

    v6UnsupportedMsg = "mini ipam driver does not handle IPv6 address pool pool requests"
    reqPoolUnsupportedMsg = "mini ipam driver does not support specific pool requests. Use default driver instead"
    unknownAsMsg = "unknown address space: %s"
    nilAllocatorMsg = "cannot make requests to the nil address space"
    brokenIdMsg = "unable to parse pool ID: %s"
    brokenIpMsg = "unable to parse ip address: %s"
    exhaustedMsg = "address space does not contain an unallocated suitable pool"
)

type driver struct {
    Local alloc.Allocator
    Global alloc.Allocator
}

func parsePools(strs []string) []*net.IPNet {
    var res []*net.IPNet
    for _, str := range strs {
        _, pool, err := net.ParseCIDR(str)
        if pool != nil && err != nil {
            res = append(res, pool)
        }
    }
    return res
}

func poolToId(as string, pool *net.IPNet) string {
    return fmt.Sprintf("%s:%s")
}

func idToPool(id string) (string, *net.IPNet) {
    m := poolIdRe.FindStringSubmatch(id)

    if len(m) == 0 {
        return "", nil
    }

    as := m[1]
    _, pool, err := net.ParseCIDR(m[2])
    if err != nil {
        return "", nil
    }

    return as, pool
}

func (d *driver) asToAllocator(as string) (alloc.Allocator, error) {
    if as == alloc.NilAS {
        return nil, types.BadRequestErrorf(nilAllocatorMsg)
    }

    if as == alloc.AddrSpace(d.Local)  {
            return d.Local, nil
    } else if as == alloc.AddrSpace(d.Global)  {
            return d.Global, nil
    } else {
        return nil, types.BadRequestErrorf(unknownAsMsg)
    }
}

func (d *driver) GetDefaultAddressSpaces() (*ipam.AddressSpacesResponse, error) {
    return &ipam.AddressSpacesResponse{ alloc.AddrSpace(d.Local), alloc.AddrSpace(d.Global) }, nil
}

func (d *driver) RequestPool(req *ipam.RequestPoolRequest) (*ipam.RequestPoolResponse, error) {
    if req.V6 {
        return nil, types.BadRequestErrorf(v6UnsupportedMsg)
    }
    if req.Pool != "" || req.SubPool != "" {
        return nil, types.BadRequestErrorf(reqPoolUnsupportedMsg)
    }

    a, err := d.asToAllocator(req.AddressSpace)
    if err != nil {
        return nil, err
    }

    pool := a.RequestPool(defaultMasklen)
    if pool == nil {
        return nil, types.BadRequestErrorf(exhaustedMsg)
    }
    return &ipam.RequestPoolResponse{ poolToId(req.AddressSpace, pool), pool.String(), nil }, nil
}

func (d *driver) ReleasePool(req *ipam.ReleasePoolRequest) error {
    as, pool := idToPool(req.PoolID)
    if pool == nil {
        return types.BadRequestErrorf(brokenIdMsg, req.PoolID)
    }

    a, err := d.asToAllocator(as)
    if err != nil {
        return err
    }

    a.ReleasePool(pool)
    return nil
}

func (d *driver) RequestAddress(req *ipam.RequestAddressRequest) (*ipam.RequestAddressResponse, error) {
    as, pool := idToPool(req.PoolID)
    if pool == nil {
        return nil, types.BadRequestErrorf(brokenIdMsg, req.PoolID)
    }

    a, err := d.asToAllocator(as)
    if err != nil {
        return nil, err
    }

    var ip net.IP
    if req.Address != "" {
        var err error
        ip, _, err = net.ParseCIDR(req.Address)
        if err != nil {
            return nil, types.BadRequestErrorf(brokenIpMsg, req.Address)
        }
    } else {
        ip = nil
    }

    ip = a.RequestAddress(pool, ip)
    if ip == nil {
        return nil, types.BadRequestErrorf(exhaustedMsg)
    }
    return &ipam.RequestAddressResponse{ ip.String(), nil }, nil
}

func (d *driver) ReleaseAddress(req *ipam.ReleaseAddressRequest) error {
    as, pool := idToPool(req.PoolID)
    if pool == nil {
        return types.BadRequestErrorf(brokenIdMsg, req.PoolID)
    }

    a, err := d.asToAllocator(as)
    if err != nil {
        return err
    }

    ip, _, err := net.ParseCIDR(req.Address)
    if err != nil || ip == nil {
        return types.BadRequestErrorf(brokenIpMsg, req.Address)
    }
    a.ReleaseAddress(ip)
    return nil
}

func (d *driver) GetCapabilities() (*ipam.CapabilitiesResponse, error) {
    return &ipam.CapabilitiesResponse{ RequiresMACAddress: false }, nil
}

func main() {
    d := &driver{ Local: alloc.NewLocalAllocator(), Global: nil }
    for _, pool := range defaultPools {
        d.Local.AddPool(pool)
    }
    h := ipam.NewHandler(d)
    h.ServeUnix(socketAddress, 0)
}
