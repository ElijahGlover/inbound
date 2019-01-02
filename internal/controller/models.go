package controller

import "crypto/tls"

// RouteTable represents a hostname from ingress
type RouteTable struct {
	Ingress string
	Host    string
	Paths   []RoutePath
}

// RoutePath represents a single route mapped to service
type RoutePath struct {
	Path        string
	ServiceName string
	ServicePort int32
}

// TLSCertificate represents a certificate
type TLSCertificate struct {
	Certificate *tls.Certificate
	SecretName  string
}

// Service represents a service endpoint
type Service struct {
	ServiceName string
	ClusterIP   string
}
