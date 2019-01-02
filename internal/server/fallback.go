package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"

	"github.com/elijahglover/inbound/internal/helpers"
)

var fallbackCertificate *tls.Certificate

func ensureFallbackCertificate() error {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	privateKeyPem, err := helpers.EncodePem("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(privateKey))
	if err != nil {
		return fmt.Errorf("Error creating tls key %s", err.Error())
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("Failed to generate serial number: %s", err.Error())
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		Subject: pkix.Name{
			CommonName: "Self Signed Certificate",
		},
		Issuer: pkix.Name{
			CommonName: "Self Signed Certificate",
		},
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certRaw, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("Failed to generate certificate: %s", err.Error())
	}
	certificatePem, err := helpers.EncodePem("CERTIFICATE", certRaw)
	if err != nil {
		return fmt.Errorf("Error creating tls key %s", err.Error())
	}

	tlsOutput, err := tls.X509KeyPair(certificatePem, privateKeyPem)
	if err != nil {
		return fmt.Errorf("Failed to generate certificate: %s", err.Error())
	}
	fallbackCertificate = &tlsOutput
	return nil
}
