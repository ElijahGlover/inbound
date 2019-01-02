package controller

import (
	"crypto/tls"
)

// GetCertificate for hostname
func (c *Controller) GetCertificate(hostname string) *tls.Certificate {
	c.certificatesLock.Lock()
	c.certificatesSecretMapLock.Lock()
	defer c.certificatesLock.Unlock()
	defer c.certificatesSecretMapLock.Unlock()

	secretName, secretOk := c.certificatesSecretMap[hostname]
	if !secretOk {
		return nil // No certificate bound to hostname
	}
	if cert, ok := c.certificates[secretName]; ok {
		return cert
	}
	return nil
}

// GetService metadata
func (c *Controller) GetService(service string) *Service {
	c.servicesLock.Lock()
	defer c.servicesLock.Unlock()

	if svc, ok := c.services[service]; ok {
		return svc
	}
	return nil
}

// GetRouteTable to resolve traffic to service
func (c *Controller) GetRouteTable(host string) *RouteTable {
	c.routeTableLock.Lock()
	defer c.routeTableLock.Unlock()

	if route, ok := c.routeTable[host]; ok {
		return route
	}
	return nil
}

// GetRouteTables returns all route tables
func (c *Controller) GetRouteTables() []*RouteTable {
	c.routeTableLock.Lock()
	defer c.routeTableLock.Unlock()

	wrappedArray := make([]*RouteTable, len(c.routeTable))
	i := 0
	for _, route := range c.routeTable {
		wrappedArray[i] = route
		i++
	}
	return wrappedArray
}

// GetServices returns all route tables
func (c *Controller) GetServices() []*Service {
	c.servicesLock.Lock()
	defer c.servicesLock.Unlock()

	wrappedArray := make([]*Service, len(c.services))
	i := 0
	for _, route := range c.services {
		wrappedArray[i] = route
		i++
	}
	return wrappedArray
}
