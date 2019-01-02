package certificates

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/elijahglover/inbound/internal/logger"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

var errChannelClosed = fmt.Errorf("Closed listening channel")

const (
	tlsPrivateKey  = "tls.key"
	tlsCertificate = "tls.crt"
)

// CertificateWatcher watches cluster
type CertificateWatcher struct {
	client                            *kubernetes.Clientset
	logger                            logger.Logger
	namespaceName                     string
	secretName                        string
	certificateChangedSubscribers     map[string]chan<- *tls.Certificate
	certificateChangedSubscribersLock *sync.Mutex
	certificateDeletedSubscribers     map[string]chan<- bool
	certificateDeletedSubscribersLock *sync.Mutex
}

// New CertificateWatcher
func New(logger logger.Logger, client *kubernetes.Clientset, namespaceName string, secretName string) *CertificateWatcher {
	return &CertificateWatcher{
		client:                            client,
		logger:                            logger,
		namespaceName:                     namespaceName,
		secretName:                        secretName,
		certificateChangedSubscribers:     map[string]chan<- *tls.Certificate{},
		certificateChangedSubscribersLock: &sync.Mutex{},
		certificateDeletedSubscribers:     map[string]chan<- bool{},
		certificateDeletedSubscribersLock: &sync.Mutex{},
	}
}

// SubscribeCertificateChanged adds channel
func (w *CertificateWatcher) SubscribeCertificateChanged(source string, add chan<- *tls.Certificate) {
	w.certificateChangedSubscribersLock.Lock()
	defer w.certificateChangedSubscribersLock.Unlock()
	w.certificateChangedSubscribers[source] = add
}

// SubscribeCertificateDeleted adds channel
func (w *CertificateWatcher) SubscribeCertificateDeleted(source string, add chan<- bool) {
	w.certificateDeletedSubscribersLock.Lock()
	defer w.certificateDeletedSubscribersLock.Unlock()
	w.certificateDeletedSubscribers[source] = add
}

func (w *CertificateWatcher) publishCertificateChanged(tls *tls.Certificate) {
	w.certificateChangedSubscribersLock.Lock()
	defer w.certificateChangedSubscribersLock.Unlock()

	if len(w.certificateChangedSubscribers) == 0 {
		return
	}

	for _, ch := range w.certificateChangedSubscribers {
		ch <- tls
	}
}

func (w *CertificateWatcher) publishCertificateDeleted(value bool) {
	w.certificateDeletedSubscribersLock.Lock()
	defer w.certificateDeletedSubscribersLock.Unlock()

	if len(w.certificateDeletedSubscribers) == 0 {
		return
	}

	for _, ch := range w.certificateDeletedSubscribers {
		ch <- value
	}
}

// Watch for change in cluster
func (w *CertificateWatcher) Watch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			err := w.watchSecret(ctx, w.namespaceName, w.secretName)
			if err != nil && err != errChannelClosed {
				w.logger.Errorf("Certificate watch returned error %s", err)
				return err
			}
		}
	}
}

func (w *CertificateWatcher) watchSecret(ctx context.Context, namespace string, secretName string) error {
	secretNamespace := w.client.Core().Secrets(namespace)

	// Filter to TLS certificates only
	secretChanges, err := secretNamespace.Watch(meta_v1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", secretName),
	})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			secretChanges.Stop()
			return nil
		case event, ok := <-secretChanges.ResultChan():
			if !ok {
				return errChannelClosed
			}
			w.processEvent(event, namespace, secretName)
		}
	}
}

func (w *CertificateWatcher) processEvent(event watch.Event, namespace string, secretName string) {
	if event.Object == nil {
		w.logger.Verbosef("Received empty payload watching secret, type %s in %s/%s", event.Type, namespace, secretName)
		return
	}
	secret := event.Object.(*v1.Secret)
	if event.Type == watch.Added || event.Type == watch.Modified {
		certificate, err := w.parseCertificate(secret.Data)
		if err != nil {
			w.logger.Errorf("Unable to load certificate %s", err)
			return
		}

		w.publishCertificateChanged(certificate)
		return
	}
	if event.Type == watch.Deleted {
		w.publishCertificateDeleted(true)
		return
	}
	w.logger.Verbosef("Received unknown message type %s watching secret %s/%s", event.Type, namespace, secretName)
}

func (w *CertificateWatcher) parseCertificate(data map[string][]byte) (*tls.Certificate, error) {
	// Check for values in bag
	if _, ok := data[tlsPrivateKey]; !ok {
		return nil, fmt.Errorf("Missing %s from secret", tlsPrivateKey)
	}
	if _, ok := data[tlsCertificate]; !ok {
		return nil, fmt.Errorf("Missing %s from secret", tlsCertificate)
	}

	certificate, err := tls.X509KeyPair(data[tlsCertificate], data[tlsPrivateKey])
	if err != nil {
		w.logger.Errorf("Unable to parse certificate %s", err)
	}
	return &certificate, err
}
