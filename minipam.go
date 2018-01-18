package main

import (
    "fmt"
    "net"
    "github.com/docker/libnetwork/types"
    "github.com/docker/go-plugins-helpers/ipam"
)

var (
    defaultAS      = "null"
    defaultPool, _ = types.ParseCIDR("0.0.0.0/0")
    defaultPoolID  = fmt.Sprintf("%s/%s", defaultAS, defaultPool.String())
)

const socketAddress = "/run/docker/plugins/mini.sock"

type allocator struct{}

func (a *allocator) GetDefaultAddressSpaces() (*ipam.AddressSpacesResponse, error) {
	return &ipam.AddressSpacesResponse{ defaultAS, defaultAS }, nil
}

func (a *allocator) RequestPool(req *ipam.RequestPoolRequest) (*ipam.RequestPoolResponse, error) {
	if req.AddressSpace != defaultAS {
		return &ipam.RequestPoolResponse{ "", "", nil }, types.BadRequestErrorf("unknown address space: %s", req.AddressSpace)
	}
	if req.Pool != "" {
		return &ipam.RequestPoolResponse{ "", "", nil }, types.BadRequestErrorf("null ipam driver does not handle specific address pool requests")
	}
	if req.SubPool != "" {
		return &ipam.RequestPoolResponse{ "", "", nil }, types.BadRequestErrorf("null ipam driver does not handle specific address subpool requests")
	}
	if req.V6 {
		return &ipam.RequestPoolResponse{ "", "", nil }, types.BadRequestErrorf("null ipam driver does not handle IPv6 address pool pool requests")
	}
        mask := net.CIDRMask(24, 32)
        ip := net.IPv4(192, 168, 20, 0)
        pool := net.IPNet{ ip, mask }
	return &ipam.RequestPoolResponse{ defaultPoolID, pool.String(), nil }, nil
}

func (a *allocator) ReleasePool(req *ipam.ReleasePoolRequest) error {
	return nil
}

func (a *allocator) RequestAddress(req *ipam.RequestAddressRequest) (*ipam.RequestAddressResponse, error) {
	if req.PoolID != defaultPoolID {
		return &ipam.RequestAddressResponse{ "", nil }, types.BadRequestErrorf("unknown pool id: %s", req.PoolID)
	}
	return &ipam.RequestAddressResponse{ "0.0.0.0/0", nil }, nil
}

func (a *allocator) ReleaseAddress(req *ipam.ReleaseAddressRequest) error {
	if req.PoolID != defaultPoolID {
		return types.BadRequestErrorf("unknown pool id: %s", req.PoolID)
	}
	return nil
}

func (a *allocator) GetCapabilities() (*ipam.CapabilitiesResponse, error) {
    return &ipam.CapabilitiesResponse{ RequiresMACAddress: false }, nil
}

func main() {
    a := &allocator{}
    h := ipam.NewHandler(a)
    h.ServeUnix(socketAddress, 0)
}
