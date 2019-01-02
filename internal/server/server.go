package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	stdlog "log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/elijahglover/inbound/internal/config"
	"github.com/elijahglover/inbound/internal/controller"
	"github.com/elijahglover/inbound/internal/helpers"
	"github.com/elijahglover/inbound/internal/logger"
	"github.com/vulcand/oxy/forward"
)

// Server component
type Server struct {
	logger     logger.Logger
	config     *config.Config
	controller *controller.Controller
	httpLogger *log.Logger        // Used to mute stdout
	fwd        *forward.Forwarder // Middleware to proxy websockets and pass host headers
}

// New server component
func New(logger logger.Logger, config *config.Config, controller *controller.Controller) *Server {
	server := &Server{
		controller: controller,
		logger:     logger,
		config:     config,
	}

	fwd, _ := forward.New(
		forward.Stream(true),
		forward.StreamingFlushInterval(100*time.Millisecond),
		forward.PassHostHeader(true),
		forward.Rewriter(&forward.HeaderRewriter{TrustForwardHeader: false}), //Don't trust anything up stream as this is internet facing
	)
	server.fwd = fwd
	server.httpLogger = stdlog.New(ioutil.Discard, "", 0)
	return server
}

// Start webserver
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting web server...")

	err := ensureFallbackCertificate()
	if err != nil {
		s.logger.Errorf("Unable to generate fallback certificate %s", err)
	} else {
		s.logger.Info("Generated fallback TLS certificate")
	}

	//Start HTTPS Server
	go func() {
		tlsConfig := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CurvePreferences: []tls.CurveID{
				tls.CurveP521,
				tls.CurveP384,
				tls.CurveP256,
			},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			},
			GetCertificate: s.resolveCertificate,
		}
		srv := &http.Server{
			Addr:         ":" + s.config.HTTPSPort,
			Handler:      http.HandlerFunc(s.handleRequest),
			TLSConfig:    tlsConfig,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0), // Forward Secrecy
			ErrorLog:     s.httpLogger,
		}
		s.logger.Infof("Listening on HTTPS 0.0.0.0:%s", s.config.HTTPSPort)
		log.Fatal(srv.ListenAndServeTLS("", ""))
	}()

	go func() {
		//Start HTTP Server
		srv := &http.Server{
			Addr:         ":" + s.config.HTTPPort,
			Handler:      http.HandlerFunc(s.handleRequest),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			ErrorLog:     s.httpLogger,
		}
		s.logger.Infof("Listening on HTTP 0.0.0.0:%s", s.config.HTTPPort)
		log.Fatal(srv.ListenAndServe())
	}()

	//Start HTTP Server - Status
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      http.HandlerFunc(s.handleStatusRequest),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     s.httpLogger,
	}
	s.logger.Infof("Listening on HTTP 0.0.0.0:8080")
	return srv.ListenAndServe()
}

func (s *Server) resolveCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert := s.controller.GetCertificate(hello.ServerName)
	if cert == nil {
		s.logger.Warningf("Missing certificate for host %s, using fallback", hello.ServerName)
		return fallbackCertificate, nil
	}
	return cert, nil
}

func (s *Server) handleRequest(w http.ResponseWriter, req *http.Request) {
	host := helpers.ExtractHostname(req.Host)
	routeTable := s.controller.GetRouteTable(host)

	//No route table found - no defined contract or the controller isn't ready
	if routeTable == nil {
		w.WriteHeader(503) // Service Unavailable
		w.Write([]byte("Service unavailable\n"))
		return
	}

	// Upgrade requests to https if http and not matching acme challenge
	if req.URL.Scheme == "http" && !strings.HasPrefix(req.URL.Path, acmeChallengeURLPrefix) {
		req.URL.Scheme = "https"
		w.Header().Set("Location", req.URL.String())
		w.WriteHeader(301)
		return
	}

	//Add HSTS
	w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")

	upstreamService := s.resolveUpstream(routeTable.Paths, req.URL)
	if upstreamService == "" {
		w.WriteHeader(404)
		w.Write([]byte("Unable to resolve service for path\n"))
		return
	}

	s.logger.Infof("Routing request to %s", upstreamService)

	// Proxy to lost
	req.URL.Scheme = "http"
	req.URL.Host = upstreamService
	s.fwd.ServeHTTP(w, req)
}

func (s *Server) resolveUpstream(routes []controller.RoutePath, url *url.URL) string {
	matchedPath := url.Path
	for _, route := range routes {
		if strings.HasPrefix(matchedPath, route.Path) {
			service := s.controller.GetService(route.ServiceName)
			if service == nil {
				s.logger.Infof("Unable to find service %s to match route %s", route.ServiceName, route.Path)
				return ""
			}
			s.logger.Verbosef("Matched path %s to service %s:%v", matchedPath, service.ClusterIP, route.ServicePort)
			return fmt.Sprintf("%s:%v", service.ClusterIP, route.ServicePort)
		}
	}
	return ""
}

func (s *Server) handleStatusRequest(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == healthcheckURL {
		w.WriteHeader(200)
		w.Write([]byte("Healthy\n"))
		return
	}

	services := s.controller.GetServices()
	routes := s.controller.GetRouteTables()

	response := healthcheckResponse{
		Services: services,
		Routes:   routes,
	}

	responseRaw, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte("Error returning status"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(responseRaw)
	return
}
