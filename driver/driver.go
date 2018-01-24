package main

import (
    "fmt"
    "net"
    "strconv"
    "regexp"
    "log"
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

func logRequest(fname string, req interface{}, res interface{}, err error) {
    if err == nil {
        log.Printf("%s(%s): %s", fname, req, res)
    } else {
        log.Printf("[FAILED] %s(%s): %s", fname, req, err)
    }
}

func logError(fname string, err error) error {
    logRequest(fname, nil, nil, err)
    return err
}

func parsePools(strs []string) []*net.IPNet {
    var res []*net.IPNet
    for _, str := range strs {
        _, pool, err := net.ParseCIDR(str)
        if pool != nil && err == nil {
            res = append(res, pool)
        }
    }
    return res
}

func poolToId(as string, pool *net.IPNet) string {
    return fmt.Sprintf("%s:%s", as, pool.String())
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

func (d *driver) GetDefaultAddressSpaces() (res *ipam.AddressSpacesResponse, err error) {
    defer func() { logRequest("GetDefaultAddressSpaces", nil, res, err) }()

    res = &ipam.AddressSpacesResponse{ alloc.AddrSpace(d.Local), alloc.AddrSpace(d.Global) }
    return res, nil
}

func (d *driver) RequestPool(req *ipam.RequestPoolRequest) (res *ipam.RequestPoolResponse, err error) {
    defer func() { logRequest("RequestPool", req, res, err) }()

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

    val, found := req.Options["CidrMaskLength"]
    var masklen int
    if found {
        masklen, err = strconv.Atoi(val)
        if err != nil {
            return nil, err
        }
    } else {
        masklen = defaultMasklen
    }

    pool, err := a.RequestPool(masklen)
    if err != nil {
        return nil, types.InternalErrorf("Allocation failed: %s", err)
    }

    res = &ipam.RequestPoolResponse{ poolToId(req.AddressSpace, pool), pool.String(), nil }
    return res, nil
}

func (d *driver) ReleasePool(req *ipam.ReleasePoolRequest) (err error) {
    defer func() { logRequest("ReleasePool", req, nil, err) }()

    as, pool := idToPool(req.PoolID)
    if pool == nil {
        return types.BadRequestErrorf(brokenIdMsg, req.PoolID)
    }

    a, err := d.asToAllocator(as)
    if err != nil {
        return err
    }

    err = a.ReleasePool(pool)
    if err != nil {
        return types.InternalErrorf("Release failed: %s", err)
    }

    return nil
}

func (d *driver) RequestAddress(req *ipam.RequestAddressRequest) (res *ipam.RequestAddressResponse, err error) {
    defer func() { logRequest("RequestAddress", req, res, err) }()

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
        ip = net.ParseIP(req.Address)
        if ip == nil {
            return nil, types.BadRequestErrorf(brokenIpMsg, req.Address)
        }
    } else {
        ip = nil
    }

    ip, err = a.RequestAddress(pool, ip)
    if err != nil {
        return nil, types.InternalErrorf("Allocation failed: %s", err)
    }

    pool.IP = ip

    res = &ipam.RequestAddressResponse{ pool.String(), nil }
    return res, nil
}

func (d *driver) ReleaseAddress(req *ipam.ReleaseAddressRequest) (err error) {
    defer func() { logRequest("ReleaseAddress", req, nil, err) }()

    as, pool := idToPool(req.PoolID)
    if pool == nil {
        return types.BadRequestErrorf(brokenIdMsg, req.PoolID)
    }

    a, err := d.asToAllocator(as)
    if err != nil {
        return err
    }

    ip := net.ParseIP(req.Address)
    if ip == nil {
        return types.BadRequestErrorf(brokenIpMsg, req.Address)
    }
    err = a.ReleaseAddress(ip)
    if err != nil {
        return types.InternalErrorf("Release failed: %s", err)
    }
    return nil
}

func (d *driver) GetCapabilities() (res *ipam.CapabilitiesResponse, err error) {
    defer func() { logRequest("GetCapabilities", nil, res, err) }()

    res = &ipam.CapabilitiesResponse{ RequiresMACAddress: false }
    return res, nil
}

func main() {
    d := &driver{ Local: alloc.NewLocalAllocator(), Global: nil }
    for _, pool := range defaultPools {
        err := d.Local.AddPool(pool)
        if err != nil {
            log.Fatalf("Failed to add pool: %s", pool.String())
        }
        log.Printf("Added pool to allocator: %s", pool.String())
    }
    h := ipam.NewHandler(d)
    h.ServeUnix(socketAddress, 0)
}
