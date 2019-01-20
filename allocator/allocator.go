package allocator

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/nategraf/mini-ipam-driver/bytop"
	"io/ioutil"
	"net"
	"os"
	"path"
	"sync"
)

// Allocator is simplified interface for managing IP addresses.
type Allocator interface {
	addrSpace() string

	AddPool(*net.IPNet) error
	RequestPool(int, *net.IPNet) (*net.IPNet, error)
	ReleasePool(*net.IPNet) error
	RequestAddress(*net.IPNet, net.IP) (net.IP, error)
	ReleaseAddress(net.IP) error
}

const NilAS = "null"

var localBackup = path.Join(os.TempDir(), "mini-ipam.gob")

func AddrSpace(a Allocator) string {
	if a == nil {
		return NilAS
	}
	return a.addrSpace()
}

// LocalAllocator is an allocator which stores data in process memory.
// It does not use an external data store and therefore cannot be used across a cluster.
type LocalAllocator struct {
	pools     [][]*net.IPNet
	allocated map[string]bool
	lock      sync.RWMutex
	update    *sync.Cond
	updated   bool
}

// NewLocalAllocator creates and initializes a new LocalAllocator
func NewLocalAllocator() *LocalAllocator {
	a := &LocalAllocator{}
	a.init()
	return a
}

// NewLocalAllocator creates and initializes a new LocalAllocator
func LoadLocalAllocator() (*LocalAllocator, error) {
	a := &LocalAllocator{}
	err := a.load()
	return a, err
}

func (a *LocalAllocator) init() {
	a.pools = make([][]*net.IPNet, 32)
	a.allocated = make(map[string]bool)
	a.lock = sync.RWMutex{}
	a.update = sync.NewCond(a.lock.RLocker())
	a.updated = false

	go a.autosave()
}

func (a *LocalAllocator) addrSpace() string {
	return "local"
}

// AddPool adds a new subnet to be used in allocations.
func (a *LocalAllocator) AddPool(pool *net.IPNet) error {
	if len(pool.Mask) != 4 {
		// This is not a proper IPv4 subnet. Abort!
		return fmt.Errorf("Only 32-bit IPv4 subnets can be added")
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	return a.addPoolNoLock(pool)
}

func (a *LocalAllocator) addPoolNoLock(pool *net.IPNet) error {
	// Operate on a normalized copy of the origonal
	pool = normalizePool(pool)

	masklen, _ := pool.Mask.Size()

	s := a.pools[masklen]
	for i, pooli := range s {
		if bytop.Equal(pool.IP, pooli.IP) {
			return fmt.Errorf("Pool has already been added: %s", pool.String())
		}
		if masklen != 0 && bytop.Equal(pool.IP, adjacentPool(pooli).IP) {
			a.pools[masklen] = append(s[:i], s[i+1:]...) // Remove the found pool from the list
			return a.addPoolNoLock(expandPool(pool))     // "Merge" the two and add the result to the allocator
		}
	}
	a.pools[masklen] = append(s, pool)
	a.signalUpdate()
	return nil
}

// RequestPool allocates a pool of the requested size.
// nil is returned if the request cannnot be fulfiled.
func (a *LocalAllocator) RequestPool(masklen int, pool *net.IPNet) (*net.IPNet, error) {
	if pool != nil {
		return nil, fmt.Errorf("LocalAllocator does not (currently) implement specific pool requests")
	}

	var i int

	if masklen < 0 || masklen > 31 {
		return nil, fmt.Errorf("Masklen must be in the interval [0, 31]")
	}

	a.lock.Lock()
	defer a.lock.Unlock()

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
	a.signalUpdate()
	return pool, nil
}

func (a *LocalAllocator) ReleasePool(pool *net.IPNet) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	if a.allocated[pool.String()] {
		a.addPoolNoLock(pool)
		delete(a.allocated, pool.String())
		a.signalUpdate()
		return nil
	} else {
		return fmt.Errorf("Pool was never allocated: %s", pool.String())
	}
}

func (a *LocalAllocator) RequestAddress(pool *net.IPNet, ip net.IP) (net.IP, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	// Make sure we allocated this pool
	if !a.allocated[pool.String()] {
		return nil, fmt.Errorf("Pool was never allocated: %s", pool.String())
	}

	// Is this a specific ip request or do we choose?
	if ip != nil {
		if pool.Contains(ip) && !a.allocated[ip.String()] {
			a.allocated[ip.String()] = true
			return ip, nil
		}

		return nil, fmt.Errorf("Cannot allocate %s from pool %s", ip.String(), pool.String())
	} else {
		ip = bytop.Copy(pool.IP.To4())
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
				a.signalUpdate()
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

	a.lock.Lock()
	defer a.lock.Unlock()

	if a.allocated[ip.String()] {
		delete(a.allocated, ip.String())
		a.signalUpdate()
		return nil
	} else {
		return fmt.Errorf("IP address was never allocated: %s", ip.String())
	}
}

func (a *LocalAllocator) Dump() map[string][]string {
	a.lock.RLock()
	defer a.lock.RUnlock()

	dump := make(map[string][]string)

	for _, s := range a.pools {
		for _, pool := range s {
			dump["free"] = append(dump["free"], pool.String())
		}
	}

	for val, _ := range a.allocated {
		dump["allocated"] = append(dump["allocated"], val)
	}

	return dump
}

// Save the allocator's current state to a file
func (a *LocalAllocator) save() error {
	dump := a.Dump()

	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(dump)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(localBackup, b.Bytes(), 0644)
}

func (a *LocalAllocator) signalUpdate() {
	a.updated = true
	a.update.Signal()
}

func (a *LocalAllocator) autosave() error {
	for {
		a.update.L.Lock()
		for !a.updated {
			a.update.Wait()
		}
		a.updated = false
		a.update.L.Unlock()

		a.save()
	}
}

// Load a saved allocator state
func (a *LocalAllocator) load() error {
	data, err := ioutil.ReadFile(localBackup)
	if err != nil {
		return err
	}

	b := bytes.Buffer{}
	b.Write(data)

	dump := make(map[string][]string)
	d := gob.NewDecoder(&b)
	err = d.Decode(&dump)
	if err != nil {
		return err
	}

	// Set this object to the initial state
	a.init()

	a.lock.Lock()
	defer a.lock.Unlock()

	// Set the allocator state to the loaded dump
	for _, str := range dump["free"] {
		_, pool, err := net.ParseCIDR(str)
		if err != nil {
			return err
		}
		pool = normalizePool(pool)
		if pool == nil {
			return fmt.Errorf("Read non-v4 IP address")
		}

		masklen, _ := pool.Mask.Size()
		a.pools[masklen] = append(a.pools[masklen], pool)
	}

	for _, str := range dump["allocated"] {
		a.allocated[str] = true
	}

	return nil
}

// Creates a copy of an ipnet, and ensures the IP component is the network address
func normalizePool(ipnet *net.IPNet) *net.IPNet {
	ip := ipnet.IP.To4()
	if ip == nil {
		return nil
	}

	ip = bytop.And(ip, ipnet.Mask, nil)
	return &net.IPNet{IP: ip, Mask: bytop.Copy(ipnet.Mask)}
}

// Given a normalized IPNet, return adjacent subnet
// The adjacent subnet is the other subnet of it's size contained in the next larger subnet
func adjacentPool(pool *net.IPNet) *net.IPNet {
	masklen, _ := pool.Mask.Size()
	if masklen <= 0 {
		return nil
	}

	ip := bytop.Copy(pool.IP)
	bytop.FlipBit(masklen-1, ip)
	return &net.IPNet{IP: ip, Mask: bytop.Copy(pool.Mask)}
}

// Expands a pool one size to fill the subnet double it's current size
func expandPool(pool *net.IPNet) *net.IPNet {
	masklen, addrlen := pool.Mask.Size()
	if masklen <= 0 {
		return nil
	}

	mask := net.CIDRMask(masklen-1, addrlen)
	ip := bytop.And(pool.IP, mask, nil)
	return &net.IPNet{IP: ip, Mask: mask}
}

func splitPool(pool *net.IPNet) (*net.IPNet, *net.IPNet) {
	masklen, addrlen := pool.Mask.Size()

	if addrlen != 32 || masklen >= 32 {
		return nil, nil
	}

	left := normalizePool(pool)
	right := normalizePool(pool)

	bytop.FlipBit(masklen, right.IP)   // Flip the bit to create a new network
	bytop.FlipBit(masklen, right.Mask) // Lengthen the mask by 1
	bytop.FlipBit(masklen, left.Mask)  // Lengthen the mask by 1

	return left, right
}

func poolOverlap(a, b *net.IPNet) bool {
	if a == nil || b == nil {
		return false
	}

	if a.Contains(bytop.And(b.IP.To4(), b.Mask, nil)) { // Check if the network addr of b is in a
		return true
	} else if b.Contains(bytop.And(a.IP.To4(), a.Mask, nil)) { // Check if the network addr of a is in b
		return true
	} else {
		return false
	}
}
