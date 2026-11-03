// Command gcp-disk-compatibility-webhook runs a Kubernetes mutating admission
// webhook that pins pods to GCP nodes compatible with the disk types of the
// volumes they mount.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cast-ai/gcp-disk-compatibility-webhook/pkg/webhook"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var rootCmd = &cobra.Command{
	Use:   "gcp-disk-compatibility-webhook",
	Short: "GCP Disk Compatibility Webhook Server",
	Long:  "A Kubernetes mutating webhook that ensures GCP disk compatibility for workloads.",
	Run:   run,
}

func init() {
	cobra.OnInitialize(initConfig)

	// Define flags
	rootCmd.Flags().Int("port", 8443, "Webhook server port")
	rootCmd.Flags().String("tls-cert-file", "/etc/webhook/certs/tls.crt", "Path to TLS certificate file")
	rootCmd.Flags().String("tls-key-file", "/etc/webhook/certs/tls.key", "Path to TLS key file")
	rootCmd.Flags().String("namespace", "disk-compatibility-webhook", "Namespace where webhook is deployed")
	rootCmd.Flags().String("service-name", "disk-compatibility-webhook", "Name of the webhook service")
	rootCmd.Flags().String("secret-name", "disk-compatibility-webhook-certs", "Name of the secret to store certificates")
	rootCmd.Flags().String("webhook-name", "disk-compatibility-webhook", "Name of the MutatingWebhookConfiguration")
	rootCmd.Flags().Bool("disable-cert-rotator", false, "Disable automatic certificate rotation")

	// Bind flags to viper
	_ = viper.BindPFlags(rootCmd.Flags())

	// Initialize klog flags
	klog.InitFlags(nil)
}

func initConfig() {
	// Set environment variable prefix
	viper.SetEnvPrefix("WEBHOOK")

	// Replace hyphens with underscores in environment variables
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Enable reading from environment variables
	viper.AutomaticEnv()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// ptr returns a pointer to the given value
func ptr[T any](v T) *T {
	return &v
}

func run(_ *cobra.Command, _ []string) {
	// Setup controller-runtime logger
	log.SetLogger(zap.New(zap.UseDevMode(true)))

	// Get configuration values from viper
	port := viper.GetInt("port")
	certFile := viper.GetString("tls-cert-file")
	keyFile := viper.GetString("tls-key-file")
	namespace := viper.GetString("namespace")
	serviceName := viper.GetString("service-name")
	secretName := viper.GetString("secret-name")
	webhookName := viper.GetString("webhook-name")
	disableCertRotator := viper.GetBool("disable-cert-rotator")

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
		// Leader election ensures only one pod manages certificates
		// while all pods can serve the webhook
		mgr, err := ctrl.NewManager(config, ctrl.Options{
			LeaderElection:          true,
			LeaderElectionID:        fmt.Sprintf("%s-cert-rotator", webhookName),
			LeaderElectionNamespace: namespace,
			LeaseDuration:           ptr(60 * time.Second),
			RenewDeadline:           ptr(50 * time.Second),
			RetryPeriod:             ptr(10 * time.Second),
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
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
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
