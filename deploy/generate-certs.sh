#!/bin/bash

# Script to generate self-signed certificates for the webhook

set -e

NAMESPACE="disk-compatibility-webhook"
SERVICE_NAME="disk-compatibility-webhook"
SECRET_NAME="${SERVICE_NAME}-certs"

# Create a temporary directory for certificates
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

echo "Generating certificates in $TEMP_DIR"

# Generate CA private key
openssl genrsa -out ca.key 2048

# Generate CA certificate
openssl req -x509 -new -nodes -key ca.key -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc" -days 365 -out ca.crt

# Generate server private key
openssl genrsa -out tls.key 2048

# Generate certificate signing request
cat >csr.conf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF

openssl req -new -key tls.key -subj "/CN=${SERVICE_NAME}.${NAMESPACE}.svc" -out tls.csr -config csr.conf

# Generate server certificate
openssl x509 -req -in tls.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out tls.crt -days 365 -extensions v3_req -extfile csr.conf

echo "Creating Kubernetes secret..."
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic "$SECRET_NAME" \
  --from-file=tls.key=tls.key \
  --from-file=tls.crt=tls.crt \
  --from-file=ca.crt=ca.crt \
  --namespace="$NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Updating webhook configuration with CA bundle..."
CA_BUNDLE=$(cat ca.crt | base64 | tr -d '\n')

# Update webhook.yaml with the CA bundle
sed -i.bak "s|caBundle: .*|caBundle: ${CA_BUNDLE}|" "$(dirname "$0")/webhook.yaml"

echo "Certificates generated and secret created successfully!"
echo "CA Bundle has been added to deploy/webhook.yaml"

# Clean up
cd -
rm -rf "$TEMP_DIR"
