package driver

import "fmt"

// ErrUnsupportedIPv6 error is returned when the driver recieves an IPv6 request.
type ErrUnsupportedIPv6 struct{}

func (e ErrUnsupportedIPv6) Error() string {
	return "IPv6 allocation requests are not supported"
}

// BadRequest denotes the type of this error
func (e ErrUnsupportedIPv6) BadRequest() {}

// ErrUnsupportedPoolReq error is returned when a caller asks for a specific address pool.
type ErrUnsupportedPoolReq struct{}

func (e ErrUnsupportedPoolReq) Error() string {
	return "specific pool requests are not supported"
}

// BadRequest denotes the type of this error
func (e ErrUnsupportedPoolReq) BadRequest() {}

// ErrAddrSpaceNotFound error is returned when a caller specifies an unknown address space.
type ErrAddrSpaceNotFound string

func (e ErrAddrSpaceNotFound) Error() string {
	return fmt.Sprintf("address space not found: %s", e)
}

// NotFound denotes the type of this error
func (e ErrAddrSpaceNotFound) NotFound() {}

// ErrNilAllocator is returned when a caller passes in the address space associated with a nil allocator.
type ErrNilAllocator struct{}

func (e ErrNilAllocator) Error() string {
	return "cannot make requests to the nil address space"
}

// BadRequest denotes the type of this error
func (e ErrNilAllocator) BadRequest() {}

// ErrParseID error is returned when an address space ID cannot be parsed.
type ErrParseID string

func (e ErrParseID) Error() string {
	return fmt.Sprintf("unable to parse pool ID: %s", e)
}

// BadRequest denotes the type of this error
func (e ErrParseID) BadRequest() {}

// ErrParseIP error is returned when an IP address cannot be parsed.
type ErrParseIP string

func (e ErrParseIP) Error() string {
	return fmt.Sprintf("unable to parse ip address: %s", e)
}

// BadRequest denotes the type of this error
func (e ErrParseIP) BadRequest() {}

// ErrAddrSpaceExhausted error is returned when there are not enough addresses in the pool for the request.
type ErrAddrSpaceExhausted struct{}

func (e ErrAddrSpaceExhausted) Error() string {
	return "address space does not contain an unallocated suitable pool"
}

// NoService denotes the type of this error
func (e ErrAddrSpaceExhausted) NoService() {}
