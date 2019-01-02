package controller

import (
	"context"
	"crypto/tls"

	certificateResource "github.com/elijahglover/inbound/internal/controller/certificates"
	ingressResource "github.com/elijahglover/inbound/internal/controller/ingress"
	namespacesResource "github.com/elijahglover/inbound/internal/controller/namespaces"
	servicesResource "github.com/elijahglover/inbound/internal/controller/services"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
)

const subscriberSource = "controller"

// Monitor cluster changes
func (c *Controller) Monitor(ctx context.Context) {
	// Only watch one namespace if configured to do so
	if c.targetNamespace != "" {
		go c.monitorNamespace(ctx, c.targetNamespace)
		return
	}

	namespaceAdded := make(chan string)
	namespaceDeleted := make(chan string)

	watcher := namespacesResource.New(c.logger, c.client)
	watcher.SubscribeNamespaceAdded(subscriberSource, namespaceAdded)
	watcher.SubscribeNamespaceDeleted(subscriberSource, namespaceDeleted)
	go watcher.Watch(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case namespaceName := <-namespaceAdded:
			// Add Namespace Watchers
			c.namespaceHandlesLock.Lock()
			if _, ok := c.namespaceHandles[namespaceName]; !ok {
				cancelCtx, cancel := context.WithCancel(ctx)
				c.namespaceHandles[namespaceName] = cancel
				go c.monitorNamespace(cancelCtx, namespaceName)
			}
			c.namespaceHandlesLock.Unlock()
		case namespaceName := <-namespaceDeleted:
			// Remove Namespace Watchers
			c.namespaceHandlesLock.Lock()
			if cancel, ok := c.namespaceHandles[namespaceName]; ok {
				delete(c.namespaceHandles, namespaceName)
				cancel()
			}
			c.namespaceHandlesLock.Unlock()
		}
	}
}

func (c *Controller) monitorNamespace(ctx context.Context, namespaceName string) {
	c.logger.Verbosef("Discovered namespace %s", namespaceName)
	ingressChanged := make(chan *v1beta1.Ingress)
	ingressDelete := make(chan *v1beta1.Ingress)

	watcher := ingressResource.New(c.logger, c.client, namespaceName)
	watcher.SubscribeIngressChanged(subscriberSource, ingressChanged)
	watcher.SubscribeIngressDeleted(subscriberSource, ingressDelete)
	go watcher.Watch(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case ingress := <-ingressChanged:
			// Add Ingress
			key := namespaceFormat(ingress.Namespace, ingress.Name)

			// Cancel existing watcher - linked certificates/services
			c.ingressHandlesLock.Lock()
			cancel, ok := c.ingressHandles[key]
			if ok {
				delete(c.ingressHandles, key)
				cancel()
			}

			cancelCtx, cancel := context.WithCancel(ctx)
			c.ingressHandles[key] = cancel
			c.ingressChanged(cancelCtx, ingress)
			c.ingressHandlesLock.Unlock()
		case ingress := <-ingressDelete:
			// Delete Ingress
			key := namespaceFormat(ingress.Namespace, ingress.Name)

			c.ingressHandlesLock.Lock()
			c.ingressDeleted(ingress)
			if cancel, ok := c.ingressHandles[key]; ok {
				delete(c.ingressHandles, key)
				cancel()
			}
			c.ingressHandlesLock.Unlock()
		}
	}
}

func (c *Controller) monitorService(ctx context.Context, namespaceName string, serviceName string) {
	serviceChanged := make(chan *v1.Service)
	serviceDeleted := make(chan string)

	watcher := servicesResource.New(c.logger, c.client, namespaceName, serviceName)
	watcher.SubscribeServiceChanged(subscriberSource, serviceChanged)
	watcher.SubscribeServiceDeleted(subscriberSource, serviceDeleted)
	go watcher.Watch(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case service := <-serviceChanged:
			c.serviceChanged(service)
		case serviceName := <-serviceDeleted:
			c.serviceDeleted(namespaceName, serviceName)
		}
	}
}

func (c *Controller) monitorCertificate(ctx context.Context, namespaceName string, secretName string) {
	certificateChanged := make(chan *tls.Certificate)
	certificateDelete := make(chan bool)

	watcher := certificateResource.New(c.logger, c.client, namespaceName, secretName)
	watcher.SubscribeCertificateChanged(subscriberSource, certificateChanged)
	watcher.SubscribeCertificateDeleted(subscriberSource, certificateDelete)
	go watcher.Watch(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case certificate := <-certificateChanged:
			c.certificateChanged(namespaceName, secretName, certificate)
		case <-certificateDelete:
			c.certificateDeleted(namespaceName, secretName)
		}
	}
}

func (c *Controller) ingressChanged(ctx context.Context, ingress *v1beta1.Ingress) {
	ingressKey := namespaceFormat(ingress.Namespace, ingress.Name)

	c.certificatesLock.Lock()
	for _, tls := range ingress.Spec.TLS {
		//Put a placeholder in certificate for certificate to attach
		for _, host := range tls.Hosts {
			c.certificatesSecretMap[host] = namespaceFormat(ingress.Namespace, tls.SecretName)
		}

		//Setup watchers for certificate changes
		go c.monitorCertificate(ctx, ingress.Namespace, tls.SecretName)
	}
	c.certificatesLock.Unlock()

	//Setup watchers for service changes
	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			go c.monitorService(ctx, ingress.Namespace, path.Backend.ServiceName)
		}
	}

	// Lock route table
	c.routeTableLock.Lock()
	defer c.routeTableLock.Unlock()

	// Process rules in ingress
	for _, rule := range ingress.Spec.Rules {
		// Check if route table exists for hostname
		if _, ok := c.routeTable[rule.Host]; !ok {
			c.routeTable[rule.Host] = &RouteTable{
				Ingress: ingress.Name,
				Host:    rule.Host,
				Paths:   make([]RoutePath, 0),
			}
		}

		ruleRouteTable := c.routeTable[rule.Host]

		for _, path := range rule.HTTP.Paths {
			serviceKey := namespaceFormat(ingress.Namespace, path.Backend.ServiceName)
			matchedPath := matchRoutePath(ruleRouteTable.Paths, path.Path)
			if matchedPath == nil {
				// Create new route path
				routePath := RoutePath{
					Path:        path.Path,
					ServiceName: serviceKey,
					ServicePort: path.Backend.ServicePort.IntVal,
				}
				ruleRouteTable.Paths = append(ruleRouteTable.Paths, routePath)
				c.logger.Verbosef("Ingress route added %s %s for host %s routes to %s:%v",
					ingressKey,
					path.Path,
					rule.Host,
					serviceKey,
					path.Backend.ServicePort.IntVal,
				)
				continue
			}

			// Update existing route path
			matchedPath.Path = path.Path
			matchedPath.ServiceName = serviceKey
			matchedPath.ServicePort = path.Backend.ServicePort.IntVal
			c.logger.Verbosef("Ingress route updated %s %s for host %s routes to %s:%v",
				ingressKey,
				path.Path,
				rule.Host,
				serviceKey,
				path.Backend.ServicePort.IntVal,
			)
		}

		// Ensure paths are sorted most complex to least complex by length
		sortRulePathsLength(ruleRouteTable.Paths)
	}
}

func (c *Controller) ingressDeleted(ingress *v1beta1.Ingress) {
	// Lock route table
	c.routeTableLock.Lock()
	defer c.routeTableLock.Unlock()

	// Process rules in ingress
	for _, rule := range ingress.Spec.Rules {
		// Check if route table exists for hostname
		if ruleRouteTable, ok := c.routeTable[rule.Host]; ok {
			for _, path := range rule.HTTP.Paths {
				ruleRouteTable.Paths = deleteRoutePath(ruleRouteTable.Paths, path.Path)
			}
		}
	}

	// TODO
	// It's difficult to know what to remove from cache with certificates and services
	// Happy path is to keep everything around for the moment
}

func (c *Controller) serviceChanged(service *v1.Service) {
	c.servicesLock.Lock()
	defer c.servicesLock.Unlock()

	key := namespaceFormat(service.Namespace, service.Name)
	c.services[key] = &Service{
		ServiceName: key,
		ClusterIP:   service.Spec.ClusterIP,
	}
	c.logger.Verbosef("Discovered service %s with cluster ip %s", key, service.Spec.ClusterIP)
}

func (c *Controller) serviceDeleted(namespace string, serviceName string) {
	key := namespaceFormat(namespace, serviceName)

	c.servicesLock.Lock()
	defer c.servicesLock.Unlock()

	if _, ok := c.services[key]; ok {
		delete(c.services, key)
	}
	c.logger.Verbosef("Removed service %s", key)
}

func (c *Controller) certificateChanged(namespace string, secretName string, tls *tls.Certificate) {
	key := namespaceFormat(namespace, secretName)

	c.certificatesLock.Lock()
	defer c.certificatesLock.Unlock()

	c.certificates[key] = tls

	c.logger.Verbosef("Discovered certificate %s", key)
}

func (c *Controller) certificateDeleted(namespace string, secretName string) {
	key := namespaceFormat(namespace, secretName)

	c.certificatesLock.Lock()
	defer c.certificatesLock.Unlock()

	if _, ok := c.certificates[key]; ok {
		delete(c.certificates, key)
	}

	c.logger.Verbosef("Removed certificate %s", key)
}
