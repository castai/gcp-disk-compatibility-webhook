package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/cast-ai/gcp-disk-compatibility-webhook/pkg/webhook"
	"k8s.io/klog/v2"
)

var (
	port     int
	certFile string
	keyFile  string
)

func init() {
	flag.IntVar(&port, "port", 8443, "Webhook server port")
	flag.StringVar(&certFile, "tls-cert-file", "/etc/webhook/certs/tls.crt", "Path to TLS certificate file")
	flag.StringVar(&keyFile, "tls-key-file", "/etc/webhook/certs/tls.key", "Path to TLS key file")
	klog.InitFlags(nil)
}

func main() {
	flag.Parse()

	handler, err := webhook.NewHandler()
	if err != nil {
		klog.Fatalf("Failed to create webhook handler: %v", err)
	}

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

	// Check if certificate files exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		klog.Fatalf("TLS certificate file not found: %s", certFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		klog.Fatalf("TLS key file not found: %s", keyFile)
	}

	if err := server.ListenAndServeTLS(certFile, keyFile); err != nil {
		klog.Fatalf("Failed to start server: %v", err)
	}
}
