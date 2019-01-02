package ingress

import (
	"context"
	"fmt"
	"sync"

	"github.com/elijahglover/inbound/internal/logger"
	"k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

var errChannelClosed = fmt.Errorf("Closed listening channel")

// IngressWatcher watches cluster
type IngressWatcher struct {
	client                        *kubernetes.Clientset
	logger                        logger.Logger
	namespaceName                 string
	ingressChangedSubscribers     map[string]chan<- *v1beta1.Ingress
	ingressChangedSubscribersLock *sync.Mutex
	ingressDeletedSubscribers     map[string]chan<- *v1beta1.Ingress
	ingressDeletedSubscribersLock *sync.Mutex
}

// New NamespacesWatcher
func New(logger logger.Logger, client *kubernetes.Clientset, namespaceName string) *IngressWatcher {
	return &IngressWatcher{
		client:                        client,
		logger:                        logger,
		namespaceName:                 namespaceName,
		ingressChangedSubscribers:     map[string]chan<- *v1beta1.Ingress{},
		ingressChangedSubscribersLock: &sync.Mutex{},
		ingressDeletedSubscribers:     map[string]chan<- *v1beta1.Ingress{},
		ingressDeletedSubscribersLock: &sync.Mutex{},
	}
}

// Watch for certificate change in cluster
func (w *IngressWatcher) Watch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			err := w.watchIngress(ctx, w.namespaceName)
			if err != nil && err != errChannelClosed {
				w.logger.Errorf("Ingress watch returned error %s", err)
				return err
			}
		}
	}
}

// SubscribeIngressChanged adds channel
func (w *IngressWatcher) SubscribeIngressChanged(source string, add chan<- *v1beta1.Ingress) {
	w.ingressChangedSubscribersLock.Lock()
	defer w.ingressChangedSubscribersLock.Unlock()
	w.ingressChangedSubscribers[source] = add
}

// SubscribeIngressDeleted adds channel
func (w *IngressWatcher) SubscribeIngressDeleted(source string, add chan<- *v1beta1.Ingress) {
	w.ingressDeletedSubscribersLock.Lock()
	defer w.ingressDeletedSubscribersLock.Unlock()
	w.ingressDeletedSubscribers[source] = add
}

func (w *IngressWatcher) publishIngressChanged(ingress *v1beta1.Ingress) {
	w.ingressChangedSubscribersLock.Lock()
	defer w.ingressChangedSubscribersLock.Unlock()

	if len(w.ingressChangedSubscribers) == 0 {
		return
	}

	for _, ch := range w.ingressChangedSubscribers {
		ch <- ingress
	}
}

func (w *IngressWatcher) publishIngressDeleted(ingress *v1beta1.Ingress) {
	w.ingressChangedSubscribersLock.Lock()
	defer w.ingressChangedSubscribersLock.Unlock()

	if len(w.ingressChangedSubscribers) == 0 {
		return
	}

	for _, ch := range w.ingressChangedSubscribers {
		ch <- ingress
	}
}

func (w *IngressWatcher) watchIngress(ctx context.Context, namespace string) error {
	ingressNamespace := w.client.Extensions().Ingresses(namespace)
	ingressChanges, err := ingressNamespace.Watch(meta_v1.ListOptions{})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			ingressChanges.Stop()
			return nil
		case event, ok := <-ingressChanges.ResultChan():
			if !ok {
				return errChannelClosed
			}
			w.processEvent(event, namespace)
		}
	}
}

func (w *IngressWatcher) processEvent(event watch.Event, namespace string) {
	if event.Object == nil {
		w.logger.Verbosef("Received empty payload watching ingresses in namespace, type %s in %s", event.Type, namespace)
		return
	}
	ingress := event.Object.(*v1beta1.Ingress)
	if event.Type == watch.Added || event.Type == watch.Modified {
		w.publishIngressChanged(ingress)
		return
	}
	if event.Type == watch.Deleted {
		w.publishIngressDeleted(ingress)
		return
	}
	w.logger.Verbosef("Received unknown message type %s watching ingresses in namespace %s", event.Type, namespace)
}
