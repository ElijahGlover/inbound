package namespaces

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

// NamespacesWatcher watche cluster
type NamespacesWatcher struct {
	client                          *kubernetes.Clientset
	logger                          logger.Logger
	namespaceAddedSubscribers       map[string]chan<- string
	namespaceAddedSubscribersLock   *sync.Mutex
	namespaceDeletedSubscribers     map[string]chan<- string
	namespaceDeletedSubscribersLock *sync.Mutex
}

// New NamespacesWatcher
func New(logger logger.Logger, client *kubernetes.Clientset) *NamespacesWatcher {
	return &NamespacesWatcher{
		client:                          client,
		logger:                          logger,
		namespaceAddedSubscribers:       map[string]chan<- string{},
		namespaceAddedSubscribersLock:   &sync.Mutex{},
		namespaceDeletedSubscribers:     map[string]chan<- string{},
		namespaceDeletedSubscribersLock: &sync.Mutex{},
	}
}

// Watch namespaces in cluster
func (w *NamespacesWatcher) Watch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			err := w.watchNamespace(ctx)
			if err != nil && err != errChannelClosed {
				w.logger.Errorf("Ingress watch returned error %s", err)
				return err
			}
		}
	}
}

// SubscribeNamespaceAdded adds channel
func (w *NamespacesWatcher) SubscribeNamespaceAdded(source string, add chan<- string) {
	w.namespaceAddedSubscribersLock.Lock()
	defer w.namespaceAddedSubscribersLock.Unlock()
	w.namespaceAddedSubscribers[source] = add
}

// SubscribeNamespaceDeleted adds channel
func (w *NamespacesWatcher) SubscribeNamespaceDeleted(source string, add chan<- string) {
	w.namespaceDeletedSubscribersLock.Lock()
	defer w.namespaceDeletedSubscribersLock.Unlock()
	w.namespaceDeletedSubscribers[source] = add
}

func (w *NamespacesWatcher) publishNamespaceAdded(name string) {
	w.namespaceAddedSubscribersLock.Lock()
	defer w.namespaceAddedSubscribersLock.Unlock()

	if len(w.namespaceAddedSubscribers) == 0 {
		return
	}

	for _, ch := range w.namespaceAddedSubscribers {
		ch <- name
	}
}

func (w *NamespacesWatcher) publishNamespaceDeleted(name string) {
	w.namespaceDeletedSubscribersLock.Lock()
	defer w.namespaceDeletedSubscribersLock.Unlock()

	if len(w.namespaceDeletedSubscribers) == 0 {
		return
	}

	for _, ch := range w.namespaceDeletedSubscribers {
		ch <- name
	}
}

func (w *NamespacesWatcher) watchNamespace(ctx context.Context) error {
	namespaces := w.client.Core().Namespaces()
	namespacesChanges, err := namespaces.Watch(meta_v1.ListOptions{})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			namespacesChanges.Stop()
			return nil
		case event, ok := <-namespacesChanges.ResultChan():
			if !ok {
				return errChannelClosed
			}
			w.processEvent(event)
		}
	}
}

func (w *NamespacesWatcher) processEvent(event watch.Event) {
	if event.Object == nil {
		w.logger.Verbosef("Received empty payload watching ingresses in namespace, type %s in %s", event.Type)
		return
	}

	namespace := event.Object.(*v1.Namespace)
	if event.Type == watch.Added || event.Type == watch.Modified {
		w.publishNamespaceAdded(namespace.Name)
		return
	}
	if event.Type == watch.Deleted {
		w.publishNamespaceDeleted(namespace.Name)
		return
	}
	w.logger.Verbosef("Received unknown message type %s watching ingresses in namespace %s", event.Type, namespace)
}
