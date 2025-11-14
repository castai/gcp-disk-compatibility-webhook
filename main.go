package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cast-ai/gcp-disk-compatibility-webhook/pkg/webhook"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	port              int
	certFile          string
	keyFile           string
	namespace         string
	serviceName       string
	secretName        string
	webhookName       string
	disableCertRotator bool
)

func init() {
	flag.IntVar(&port, "port", 8443, "Webhook server port")
	flag.StringVar(&certFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "Path to TLS certificate file")
	flag.StringVar(&keyFile, "tls-key-file", "/etc/webhook/certs/tls.key", "Path to TLS key file")
	flag.StringVar(&namespace, "namespace", "disk-compatibility-webhook", "Namespace where webhook is deployed")
	flag.StringVar(&serviceName, "service-name", "disk-compatibility-webhook", "Name of the webhook service")
	flag.StringVar(&secretName, "secret-name", "disk-compatibility-webhook-certs", "Name of the secret to store certificates")
	flag.StringVar(&webhookName, "webhook-name", "disk-compatibility-webhook", "Name of the MutatingWebhookConfiguration")
	flag.BoolVar(&disableCertRotator, "disable-cert-rotator", false, "Disable automatic certificate rotation")
	klog.InitFlags(nil)
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	// Initialize Kubernetes client config
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to get in-cluster config: %v", err)
	}

	// Setup certificate rotator if enabled
	if !disableCertRotator {
		klog.Info("Setting up certificate rotator")

		// Create controller-runtime manager for cert rotation
		mgr, err := ctrl.NewManager(config, ctrl.Options{
			LeaderElection: false,
			Metrics: server.Options{
				BindAddress: "0", // Disable metrics
			},
		})
		if err != nil {
			klog.Fatalf("Failed to create controller manager: %v", err)
		}

		// Setup webhook info
		webhooks := []rotator.WebhookInfo{
			{
				Name: webhookName,
				Type: rotator.Mutating,
			},
		}

		// Channel to signal when certificates are ready
		certsReady := make(chan struct{})

		// Create and add certificate rotator
		if err := rotator.AddRotator(mgr, &rotator.CertRotator{
			SecretKey: types.NamespacedName{
				Namespace: namespace,
				Name:      secretName,
			},
			CertDir:        "/etc/webhook/certs",
			CertName:       "tls.crt",
			KeyName:        "tls.key",
			CAName:         "ca.crt",
			CAOrganization: "cast.ai",
			DNSName:        fmt.Sprintf("%s.%s.svc", serviceName, namespace),
			ExtraDNSNames:  []string{fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)},
			IsReady:        certsReady,
			Webhooks:       webhooks,
		}); err != nil {
			klog.Fatalf("Failed to add certificate rotator: %v", err)
		}

		// Start the manager in a goroutine
		go func() {
			klog.Info("Starting certificate rotator manager")
			if err := mgr.Start(ctx); err != nil {
				klog.Fatalf("Failed to start certificate rotator: %v", err)
			}
		}()

		// Wait for certificates to be ready
		klog.Info("Waiting for certificates to be ready...")
		<-certsReady
		klog.Info("Certificates are ready")
	} else {
		klog.Info("Certificate rotator disabled")

		// Check if certificate files exist when rotator is disabled
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			klog.Fatalf("TLS certificate file not found: %s", certFile)
		}
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			klog.Fatalf("TLS key file not found: %s", keyFile)
		}
	}

	// Create webhook handler
	handler, err := webhook.NewHandler()
	if err != nil {
		klog.Fatalf("Failed to create webhook handler: %v", err)
	}

	// Setup HTTP handlers
	http.HandleFunc("/mutate", handler.Handle)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%d", port)
	klog.Infof("Starting webhook server on %s", addr)

	server := &http.Server{
		Addr: addr,
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("Failed to start server: %v", err)
		}
	}()

	klog.Info("Webhook server started successfully")

	// Wait for termination signal
	<-signalCh
	klog.Info("Received termination signal, shutting down gracefully...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("Error during server shutdown: %v", err)
	}

	klog.Info("Server shutdown complete")
}
