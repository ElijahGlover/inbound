package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/elijahglover/inbound/internal/config"
	"github.com/elijahglover/inbound/internal/controller"
	"github.com/elijahglover/inbound/internal/logger"
	"github.com/elijahglover/inbound/internal/server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func createK8sClient(logger logger.Logger, config *config.Config) (*kubernetes.Clientset, error) {
	//Use in cluster config
	if config.KubeConfig == "" {
		logger.Infof("Connecting using in cluster configuration")
		clientConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, logger.Errorf("Error connecting to cluster %s", err)
		}
		clientSet, err := kubernetes.NewForConfig(clientConfig)
		if err != nil {
			return nil, logger.Errorf("Error connecting to cluster %s", err)
		}
		return clientSet, nil
	}

	//Use out of cluster test using kubeconfig
	logger.Infof("Connecting using config from path %s", config.KubeConfig)
	clientConfig, err := clientcmd.BuildConfigFromFlags("", config.KubeConfig)
	if err != nil {
		return nil, logger.Errorf("Error connecting to cluster %s", err)
	}

	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, logger.Errorf("Error connecting to cluster %s", err)
	}
	return clientSet, nil
}

func execute(ctx context.Context) error {
	configComp, err := config.FromEnv()
	if err != nil {
		return fmt.Errorf("Error with configuration %s", err)
	}

	loggerComp := logger.NewStdOut(configComp)
	k8sClient, err := createK8sClient(loggerComp, configComp)
	if err != nil {
		return err
	}

	// K8s Resource/Controller Watcher
	controllerComp := controller.New(loggerComp, configComp.TargetNamespace, k8sClient)
	go controllerComp.Monitor(ctx)

	serverComp := server.New(loggerComp, configComp, controllerComp)
	return serverComp.Start(ctx)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// Listen for interrupt
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Println("Shutting down application...")
		cancel()
		fmt.Println("Application shutdown safely from interrupt")
		os.Exit(0)
	}()

	// Execute application
	err := execute(ctx)
	cancel()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
