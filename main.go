package main

import (
	"github.com/docker/go-plugins-helpers/ipam"
	"github.com/nategraf/mini-ipam-driver/allocator"
	"github.com/nategraf/mini-ipam-driver/driver"
	"github.com/sirupsen/logrus"
)

const socketAddress = "/run/docker/plugins/mini.sock"

func main() {
	a, err := allocator.LoadLocalAllocator()
	if err == nil {
		logrus.Infof("Successfully loaded allocator state")
		dump := a.Dump()
		logrus.Infof("Free pools: %s", dump["free"])
		logrus.Infof("Allocated: %s", dump["allocated"])
	} else {
		logrus.Infof("Failed to load allocator state from file: %s", err)

		a = allocator.NewLocalAllocator()
		for _, pool := range driver.DefaultPools {
			err := a.AddPool(pool)
			if err != nil {
				logrus.Fatalf("Failed to add pool: %s", pool.String())
			}
			logrus.Infof("Added pool to allocator: %s", pool.String())
		}
	}

	d := &driver.Driver{Local: a, Global: nil}
	h := ipam.NewHandler(d)
	h.ServeUnix(socketAddress, 0)
}
