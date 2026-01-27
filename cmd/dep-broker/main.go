package main

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/pem"
    "flag"
    "fmt"
    "io"
    "log"
    "math/big"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "github.com/smallstep/pkcs7"
)

// dep-broker is a lightweight sidecar exposing minimal endpoints
// to complete the ABM DEP/ADE token PKI exchange without external deps.
// It generates a certificate for ABM and decrypts the .p7m server token.
func main() {
    listen := envOr("DEP_BROKER_LISTEN", ":9010")
    storageDir := envOr("DEP_BROKER_STORAGE", "./dep_broker_data")

    if err := os.MkdirAll(storageDir, 0o755); err != nil {
        log.Fatalf("failed to create storage dir: %v", err)
    }

    mux := http.NewServeMux()

    mux.HandleFunc("/dep/token/getcert", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        certPEM, keyPEM, err := generateCert()
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        if err := os.WriteFile(filepath.Join(storageDir, "token_key.pem"), keyPEM, 0o600); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        if err := os.WriteFile(filepath.Join(storageDir, "token_cert.pem"), certPEM, 0o644); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/x-pem-file")
        _, _ = w.Write(certPEM)
    })

    mux.HandleFunc("/dep/token/decrypt", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        body, err := io.ReadAll(r.Body)
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        keyPEM, err := os.ReadFile(filepath.Join(storageDir, "token_key.pem"))
        if err != nil {
            http.Error(w, "private key not found; call /dep/token/getcert first", http.StatusPreconditionFailed)
            return
        }
        block, _ := pem.Decode(keyPEM)
        if block == nil {
            http.Error(w, "invalid private key PEM", http.StatusInternalServerError)
            return
        }
        priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        certPEM, err := os.ReadFile(filepath.Join(storageDir, "token_cert.pem"))
        if err != nil {
            http.Error(w, "certificate not found; call /dep/token/getcert first", http.StatusPreconditionFailed)
            return
        }
        certBlock, _ := pem.Decode(certPEM)
        if certBlock == nil {
            http.Error(w, "invalid certificate PEM", http.StatusInternalServerError)
            return
        }
        cert, err := x509.ParseCertificate(certBlock.Bytes)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        p7, err := pkcs7.Parse(body)
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        decrypted, err := p7.Decrypt(cert, priv)
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        if err := os.WriteFile(filepath.Join(storageDir, "server_token.json"), decrypted, 0o600); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write(decrypted)
    })

    mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "text/plain")
        _, _ = w.Write([]byte("ok"))
    })

    mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "text/plain")
        _, _ = w.Write([]byte(usage(storageDir)))
    })

    flag.Parse()
    log.Printf("dep-broker listening on %s (storage=%s)", listen, storageDir)
    if err := http.ListenAndServe(listen, mux); err != nil {
        log.Fatal(err)
    }
}

func generateCert() (certPEM, keyPEM []byte, err error) {
    key, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return nil, nil, err
    }
    tpl := &x509.Certificate{
        SerialNumber: big.NewInt(time.Now().UnixNano()),
        Subject: pkix.Name{
            CommonName:   "DEP Broker Token",
            Organization: []string{"NanoMDM"},
        },
        NotBefore:             time.Now().Add(-5 * time.Minute),
        NotAfter:              time.Now().Add(365 * 24 * time.Hour),
        KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment | x509.KeyUsageDigitalSignature,
        BasicConstraintsValid: true,
    }
    der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
    if err != nil {
        return nil, nil, err
    }
    certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
    keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
    return certPEM, keyPEM, nil
}

func envOr(k, def string) string {
    if v := os.Getenv(k); v != "" {
        return v
    }
    return def
}

func usage(storage string) string {
    return fmt.Sprintf(`DEP Broker

Endpoints:
  POST /dep/token/getcert   -> returns a PEM certificate to upload in ABM when creating the MDM Server
  POST /dep/token/decrypt   -> accepts ABM .p7m Server Token (binary body) and stores decrypted JSON in %s

Environment:
  DEP_BROKER_LISTEN   default :9010
  DEP_BROKER_STORAGE  default ./dep_broker_data

Workflow:
  1. Call POST /dep/token/getcert and upload the returned cert to ABM when adding an MDM Server.
  2. Download the Server Token (.p7m) from ABM and POST its raw bytes to /dep/token/decrypt.
  3. The decrypted token JSON will be saved under DEP_BROKER_STORAGE; use it with a DEP tool
     (e.g., MicroMDM or future automation) to create/assign ADE profiles that point to NanoMDM.
`, storage)
}


