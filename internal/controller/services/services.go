package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/elijahglover/inbound/internal/logger"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

var errChannelClosed = fmt.Errorf("Closed listening channel")

// ServiceWatcher watches cluster
type ServiceWatcher struct {
	client                        *kubernetes.Clientset
	logger                        logger.Logger
	namespaceName                 string
	serviceName                   string
	serviceChangedSubscribers     map[string]chan<- *v1.Service
	serviceChangedSubscribersLock *sync.Mutex
	serviceDeletedSubscribers     map[string]chan<- string
	serviceDeletedSubscribersLock *sync.Mutex
}

// New CertificateWatcher
func New(logger logger.Logger, client *kubernetes.Clientset, namespaceName string, serviceName string) *ServiceWatcher {
	return &ServiceWatcher{
		client:                        client,
		logger:                        logger,
		namespaceName:                 namespaceName,
		serviceName:                   serviceName,
		serviceChangedSubscribers:     map[string]chan<- *v1.Service{},
		serviceChangedSubscribersLock: &sync.Mutex{},
		serviceDeletedSubscribers:     map[string]chan<- string{},
		serviceDeletedSubscribersLock: &sync.Mutex{},
	}
}

// Watch for service change in cluster
func (w *ServiceWatcher) Watch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			err := w.watchService(ctx, w.namespaceName, w.serviceName)
			if err != nil && err != errChannelClosed {
				w.logger.Errorf("Service watch returned error %s", err)
				return err
			}
		}
	}
}

// SubscribeServiceChanged adds channel
func (w *ServiceWatcher) SubscribeServiceChanged(source string, add chan<- *v1.Service) {
	w.serviceChangedSubscribersLock.Lock()
	defer w.serviceChangedSubscribersLock.Unlock()
	w.serviceChangedSubscribers[source] = add
}

// SubscribeServiceDeleted adds channel
func (w *ServiceWatcher) SubscribeServiceDeleted(source string, add chan<- string) {
	w.serviceDeletedSubscribersLock.Lock()
	defer w.serviceDeletedSubscribersLock.Unlock()
	w.serviceDeletedSubscribers[source] = add
}

func (w *ServiceWatcher) publishCertificateChanged(service *v1.Service) {
	w.serviceDeletedSubscribersLock.Lock()
	defer w.serviceDeletedSubscribersLock.Unlock()

	if len(w.serviceChangedSubscribers) == 0 {
		return
	}

	for _, ch := range w.serviceChangedSubscribers {
		ch <- service
	}
}

func (w *ServiceWatcher) publishCertificateDeleted(name string) {
	w.serviceDeletedSubscribersLock.Lock()
	defer w.serviceDeletedSubscribersLock.Unlock()

	if len(w.serviceDeletedSubscribers) == 0 {
		return
	}

	for _, ch := range w.serviceDeletedSubscribers {
		ch <- name
	}
}

func (w *ServiceWatcher) watchService(ctx context.Context, namespace string, serviceName string) error {
	serviceNamespace := w.client.Core().Services(namespace)

	serviceChanges, err := serviceNamespace.Watch(meta_v1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", serviceName),
	})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			serviceChanges.Stop()
			return nil
		case event, ok := <-serviceChanges.ResultChan():
			if !ok {
				return errChannelClosed
			}
			w.processEvent(event, namespace, serviceName)
		}
	}
}

func (w *ServiceWatcher) processEvent(event watch.Event, namespace string, serviceName string) {
	if event.Object == nil {
		w.logger.Verbosef("Received empty payload watching service, %s type in %s/%s", event.Type, namespace, serviceName)
		return
	}

	service := event.Object.(*v1.Service)
	if event.Type == watch.Added || event.Type == watch.Modified {
		w.publishCertificateChanged(service)
		return
	}
	if event.Type == watch.Deleted {
		w.publishCertificateDeleted(service.Name)
		return
	}
	w.logger.Verbosef("Received unknown message type %s watching service %s/%s", event.Type, namespace, serviceName)
}
