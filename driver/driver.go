package driver

import (
	"fmt"
	"net"
	"reflect"
	"regexp"
	"strconv"

	"github.com/docker/go-plugins-helpers/ipam"
	"github.com/docker/libnetwork/types"
	"github.com/nategraf/mini-ipam-driver/allocator"
	"github.com/sirupsen/logrus"
)

var (
	// DefaultPools are the IP blocks used when no others are provided.
	DefaultPools = parsePools([]string{"172.16.0.0/16"})

	poolIdRe = regexp.MustCompile("([a-zA-Z0-9_]+):([a-zA-Z0-9./]+)")
)

// DefaultMaskLength specifies the CIDR mask length to use if one is not specified.
const DefaultMaskLength = 28

type Driver struct {
	Local  allocator.Allocator
	Global allocator.Allocator
}

// unwrap gives the pointed to value if the i is an non-nil pointer.
func unwrap(i interface{}) interface{} {
	if v := reflect.ValueOf(i); v.Kind() == reflect.Ptr && !v.IsNil() {
		return v.Elem()
	}
	return i
}

// logRequest logs request inputs and results.
func logRequest(fname string, req interface{}, res interface{}, err error) {
	req, res = unwrap(req), unwrap(res)
	if err == nil {
		if res == nil {
			logrus.Infof("%s(%v)", fname, req)
		} else {
			logrus.Infof("%s(%v): %v", fname, req, res)
		}
		return
	}
	switch err.(type) {
	case types.MaskableError:
		logrus.WithError(err).Infof("[MaskableError] %s(%v): %v", fname, req, err)
	case types.RetryError:
		logrus.WithError(err).Infof("[RetryError] %s(%v): %v", fname, req, err)
	case types.BadRequestError:
		logrus.WithError(err).Warnf("[BadRequestError] %s(%v): %v", fname, req, err)
	case types.NotFoundError:
		logrus.WithError(err).Warnf("[NotFoundError] %s(%v): %v", fname, req, err)
	case types.ForbiddenError:
		logrus.WithError(err).Warnf("[ForbiddenError] %s(%v): %v", fname, req, err)
	case types.NoServiceError:
		logrus.WithError(err).Warnf("[NoServiceError] %s(%v): %v", fname, req, err)
	case types.NotImplementedError:
		logrus.WithError(err).Warnf("[NotImplementedError] %s(%v): %v", fname, req, err)
	case types.TimeoutError:
		logrus.WithError(err).Errorf("[TimeoutError] %s(%v): %v", fname, req, err)
	case types.InternalError:
		logrus.WithError(err).Errorf("[InternalError] %s(%v): %v", fname, req, err)
	default:
		// Unclassified errors should be treated as bad.
		logrus.WithError(err).Errorf("[UNKNOWN] %s(%v): %v", fname, req, err)
	}
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

func (d *Driver) asToAllocator(as string) (allocator.Allocator, error) {
	switch as {
	case allocator.NilAS:
		return nil, ErrNilAllocator{}
	case allocator.AddrSpace(d.Local):
		return d.Local, nil
	case allocator.AddrSpace(d.Global):
		return d.Global, nil
	default:
		return nil, ErrAddrSpaceNotFound(as)
	}
}

func (d *Driver) GetDefaultAddressSpaces() (res *ipam.AddressSpacesResponse, err error) {
	defer func() { logRequest("GetDefaultAddressSpaces", nil, res, err) }()

	res = &ipam.AddressSpacesResponse{allocator.AddrSpace(d.Local), allocator.AddrSpace(d.Global)}
	return res, nil
}

func (d *Driver) RequestPool(req *ipam.RequestPoolRequest) (res *ipam.RequestPoolResponse, err error) {
	defer func() { logRequest("RequestPool", req, res, err) }()

	if req.V6 {
		return nil, ErrUnsupportedIPv6{}
	}
	if req.Pool != "" || req.SubPool != "" {
		return nil, ErrUnsupportedPoolReq{}
	}

	a, err := d.asToAllocator(req.AddressSpace)
	if err != nil {
		return nil, err
	}

	val, found := req.Options[CidrMaskLength]
	var masklen int
	if found {
		masklen, err = strconv.Atoi(val)
		if err != nil {
			return nil, err
		}
	} else {
		masklen = DefaultMaskLength
	}

	pool, err := a.RequestPool(masklen, nil)
	if err != nil {
		return nil, types.InternalErrorf("Allocation failed: %s", err)
	}

	res = &ipam.RequestPoolResponse{poolToId(req.AddressSpace, pool), pool.String(), nil}
	return res, nil
}

func (d *Driver) ReleasePool(req *ipam.ReleasePoolRequest) (err error) {
	defer func() { logRequest("ReleasePool", req, nil, err) }()

	as, pool := idToPool(req.PoolID)
	if pool == nil {
		return ErrParseID(req.PoolID)
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

func (d *Driver) RequestAddress(req *ipam.RequestAddressRequest) (res *ipam.RequestAddressResponse, err error) {
	defer func() { logRequest("RequestAddress", req, res, err) }()

	as, pool := idToPool(req.PoolID)
	if pool == nil {
		return nil, ErrParseID(req.PoolID)
	}

	a, err := d.asToAllocator(as)
	if err != nil {
		return nil, err
	}

	var ip net.IP
	if req.Address != "" {
		ip = net.ParseIP(req.Address)
		if ip == nil {
			return nil, ErrParseIP(req.Address)
		}
	} else {
		ip = nil
	}

	ip, err = a.RequestAddress(pool, ip)
	if err != nil {
		return nil, types.InternalErrorf("Allocation failed: %s", err)
	}

	pool.IP = ip

	res = &ipam.RequestAddressResponse{pool.String(), nil}
	return res, nil
}

func (d *Driver) ReleaseAddress(req *ipam.ReleaseAddressRequest) (err error) {
	defer func() { logRequest("ReleaseAddress", req, nil, err) }()

	as, pool := idToPool(req.PoolID)
	if pool == nil {
		return ErrParseID(req.PoolID)
	}

	a, err := d.asToAllocator(as)
	if err != nil {
		return err
	}

	ip := net.ParseIP(req.Address)
	if ip == nil {
		return ErrParseIP(req.Address)
	}
	err = a.ReleaseAddress(ip)
	if err != nil {
		return types.InternalErrorf("Release failed: %s", err)
	}
	return nil
}

func (d *Driver) GetCapabilities() (res *ipam.CapabilitiesResponse, err error) {
	defer func() { logRequest("GetCapabilities", nil, res, err) }()

	res = &ipam.CapabilitiesResponse{RequiresMACAddress: false}
	return res, nil
}
