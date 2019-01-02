package controller

import (
	"context"
	"crypto/tls"
	"sync"

	"github.com/elijahglover/inbound/internal/logger"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	logger          logger.Logger
	targetNamespace string
	// k8s client
	client *kubernetes.Clientset
	// TLS Loaded Cache - key = secret name, value is certificate
	certificates     map[string]*tls.Certificate
	certificatesLock *sync.Mutex
	// TLS active certificates - Key = hostname and value is certificate with metadata
	certificatesSecretMap     map[string]string
	certificatesSecretMapLock *sync.Mutex
	// Registry of all services defined, key is service name, value is service metadata
	services     map[string]*Service
	servicesLock *sync.Mutex
	// Route table, key is hostname, value is route table
	routeTable     map[string]*RouteTable
	routeTableLock *sync.Mutex
	// Namespace - required for duplicate events fired by k8s
	namespaceHandles     map[string]context.CancelFunc
	namespaceHandlesLock *sync.Mutex
	// Ingress - required for duplicate events fired by k8s
	ingressHandles     map[string]context.CancelFunc
	ingressHandlesLock *sync.Mutex
}

// New controller
func New(logger logger.Logger, targetNamespace string, client *kubernetes.Clientset) *Controller {
	return &Controller{
		logger:                    logger,
		targetNamespace:           targetNamespace,
		client:                    client,
		certificates:              map[string]*tls.Certificate{},
		certificatesLock:          &sync.Mutex{},
		certificatesSecretMap:     map[string]string{},
		certificatesSecretMapLock: &sync.Mutex{},
		services:                  map[string]*Service{},
		servicesLock:              &sync.Mutex{},
		routeTable:                map[string]*RouteTable{},
		routeTableLock:            &sync.Mutex{},
		namespaceHandles:          map[string]context.CancelFunc{},
		namespaceHandlesLock:      &sync.Mutex{},
		ingressHandles:            map[string]context.CancelFunc{},
		ingressHandlesLock:        &sync.Mutex{},
	}
}
