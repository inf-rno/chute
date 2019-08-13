package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"os"
	"sync"
	"time"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	"github.com/envoyproxy/go-control-plane/pkg/test"
	"github.com/sirupsen/logrus"
)

// Hasher returns node ID as an ID
type Hasher struct {
}

// ID function
func (h Hasher) ID(node *core.Node) string {
	if node == nil {
		return "unknown"
	}
	return node.Id
}

func main() {
	ctx := context.Background()
	logger := logrus.WithFields(logrus.Fields{
		"chute": true,
	})

	// create a config cache
	signal := make(chan struct{})
	cb := &callbacks{signal: signal}
	config := cache.NewSnapshotCache(false, Hasher{}, logger)
	srv := server.NewServer(config, cb)
	als := &test.AccessLogService{}

	// start the xDS server
	go test.RunAccessLogServer(ctx, als, 8888)
	go test.RunManagementServer(ctx, srv, 18000)
	go test.RunManagementGateway(ctx, srv, 18001)

	snapshot := cache.Snapshot{
		Secrets: cache.NewResources("v1", makeSecrets()),
	}
	if err := snapshot.Consistent(); err != nil {
		log.Printf("snapshot inconsistency: %+v\n", snapshot)
	}

	err := config.SetSnapshot("ingress-envoy", snapshot)
	if err != nil {
		log.Printf("snapshot error %q for %+v\n", err, snapshot)
		os.Exit(1)
	}

	err = config.SetSnapshot("app1-envoy", snapshot)
	if err != nil {
		log.Printf("snapshot error %q for %+v\n", err, snapshot)
		os.Exit(1)
	}

	err = config.SetSnapshot("app2-envoy", snapshot)
	if err != nil {
		log.Printf("snapshot error %q for %+v\n", err, snapshot)
		os.Exit(1)
	}

	select {}

	als.Dump(func(s string) {
		log.Println(s)
	})
	cb.Report()
}

type callbacks struct {
	signal   chan struct{}
	fetches  int
	requests int
	mu       sync.Mutex
}

func (cb *callbacks) Report() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	log.Printf("server callbacks fetches=%d requests=%d\n", cb.fetches, cb.requests)
}
func (cb *callbacks) OnStreamOpen(_ context.Context, id int64, typ string) error {
	log.Printf("stream %d open for %s\n", id, typ)
	return nil
}
func (cb *callbacks) OnStreamClosed(id int64) {
	log.Printf("stream %d closed\n", id)
}
func (cb *callbacks) OnStreamRequest(int64, *v2.DiscoveryRequest) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.requests++
	if cb.signal != nil {
		close(cb.signal)
		cb.signal = nil
	}
	return nil
}
func (cb *callbacks) OnStreamResponse(int64, *v2.DiscoveryRequest, *v2.DiscoveryResponse) {}
func (cb *callbacks) OnFetchRequest(_ context.Context, req *v2.DiscoveryRequest) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.fetches++
	if cb.signal != nil {
		close(cb.signal)
		cb.signal = nil
	}
	return nil
}
func (cb *callbacks) OnFetchResponse(*v2.DiscoveryRequest, *v2.DiscoveryResponse) {}

const (
	caCert = `-----BEGIN CERTIFICATE-----
MIID0jCCArqgAwIBAgIJAJxgLCQiz7YlMA0GCSqGSIb3DQEBCwUAMHYxCzAJBgNV
BAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1TYW4gRnJhbmNp
c2NvMQ0wCwYDVQQKDARMeWZ0MRkwFwYDVQQLDBBMeWZ0IEVuZ2luZWVyaW5nMRAw
DgYDVQQDDAdUZXN0IENBMB4XDTE4MTIxNzIwMTgwMFoXDTIwMTIxNjIwMTgwMFow
djELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNh
biBGcmFuY2lzY28xDTALBgNVBAoMBEx5ZnQxGTAXBgNVBAsMEEx5ZnQgRW5naW5l
ZXJpbmcxEDAOBgNVBAMMB1Rlc3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw
ggEKAoIBAQCpZHOUq+nidd+Gz44RC80QG9De9jcFUStEMGucXlnvvp2cH3GV4GmO
IZPdCwasxuruO3VM/Yt8tUAO2OrTQayuL9GXTt8MTpkCrviebMBzjYjbgyLgDpZy
cMoEJjBx0JsfQV+9IUDROLlIehTYzjcIWuLEOqMjZXQQCOI+jA3CEWZx1TFhRWhi
9aBnQQzWCSZPV5ErKSSRg2T2Xnuhusue7ETtgSt36hDrOxLhJaeS1/YlovyhX94j
JPhASK3LutJUDO2tk8L713Y3WHkFzDMfkGrklRbBB/ZKXRRGiJDWElpbUCUVFbuw
7laBtTn0t74DQxBXqal9sIr9vV7LLQszAgMBAAGjYzBhMA8GA1UdEwEB/wQFMAMB
Af8wDgYDVR0PAQH/BAQDAgEGMB0GA1UdDgQWBBQUM9b2kmz7njy/vuxkzKiwDLZN
5DAfBgNVHSMEGDAWgBQUM9b2kmz7njy/vuxkzKiwDLZN5DANBgkqhkiG9w0BAQsF
AAOCAQEAkWqORue+2exZldWbYaDUX3ANP0ATBNIbZM70uTNO8Iy+r5Fvxtae/PsV
Iac9LzVY5dY5eqIND9wo7osFfxEhJdJn+/tpTU2h9IhsuWMm0Ogj87NS3sy0xwDc
xBhnVXI8nCDYU3qU3p+AeC0VfEbNb+dRKHH/FL77jvIL64GP/WGxxS9u7LRTUUoR
g97ZWeayKEsHAicRao4/k3jgpNIUN0BOlkjLvCe1ExU0id5R3UtdITmbuSSe6GJx
j8xsEV8PxmOIaJ/M+fqE+Zi2Ljp3a+9X/nLakR6ohMNTbrGMQWrGIpFqCj6pIwek
6Uemmmca+JeVohl8P3enMlW1d6/24w==
-----END CERTIFICATE-----`

	caKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAqWRzlKvp4nXfhs+OEQvNEBvQ3vY3BVErRDBrnF5Z776dnB9x
leBpjiGT3QsGrMbq7jt1TP2LfLVADtjq00Gsri/Rl07fDE6ZAq74nmzAc42I24Mi
4A6WcnDKBCYwcdCbH0FfvSFA0Ti5SHoU2M43CFrixDqjI2V0EAjiPowNwhFmcdUx
YUVoYvWgZ0EM1gkmT1eRKykkkYNk9l57obrLnuxE7YErd+oQ6zsS4SWnktf2JaL8
oV/eIyT4QEity7rSVAztrZPC+9d2N1h5BcwzH5Bq5JUWwQf2Sl0URoiQ1hJaW1Al
FRW7sO5WgbU59Le+A0MQV6mpfbCK/b1eyy0LMwIDAQABAoIBAHFGvYwkUqmgTbRn
RAfeLmmhUFJpsG2b1CUrhCrzZY1PmTJ4TIr/oVbs2WauIu6TrzNVC6JKw2bIBmhn
YtGXT5TEYZKfqcUfIm+K9rNq4l/jvCufTEktOCqbhlyz9R2HdNS38QAXJrNDDZSM
HzjE3kR2EsNKuyHGjJDUgAd3vROTXLuNxhfO+he84NTz6hPlzNOGRJDOnacIp1T1
qbQdlHhqxJBVyWyjAYR38maBrleLZqV27Fd0sBzuUkU7i6vHRFHp4pD4PAzkzAsS
DMqCmVF2cM83BT9qz8TcP4wd4+hj9OG2QbIe1zfhUfYS8v/bNqwtIV4WlbyW3P65
ynXSaAECgYEA0XptQYYrPeSLkEQNLPfDu4BnTE4X65exl8c5+qMWg3aECbdaxWqi
VmjNlDyzz0w3zzSRShNR7/6fSWULrWwdSCRbpqiU1+xSUrPXCT+tqI9CVqVl452E
rGHiPZgy7ljb1x/mfrhA8fASrp57Xze4DLqYEUI5fiYBcJRhVqGZvTMCgYEAzwMA
+wbe2qyi9CuiEfDY3sv5+gfLkYUh3yR7dwrdqccAnOgG0nOv4LDwaoW4Fk2aAYdw
GvutdUntXtVc77Ha613cXCL38w58r890EpQuDoSTvZlo6K7lrB0ZFpUdefvpP5eL
4hrkRXes4TeshjLT8C+0Fo7+2XbZfWx2ZHrzegECgYEAh3rYwrIVsXfo06tPoi+0
RcZsCKvRSKvZTkKpuvJTkz7JcsdFS70FtUEfBKql2IKA7eAfv3rzWXaiaoORo93y
qj/pjsYlTekn7RknEHJAzG2rCAL8/NNZhWvhONkAx6pstJuLJZXhWxhb3NffDtwo
iwL7at4b9Px7neY5diAaIIUCgYA/OXujL4YA45khWfI16IlUAphmdNsHptGhhVLw
GLF6mPzm7zamMA8XYPMMlaqTpT/UF7l1hEiF+f41aJTp4Dgsio4y1btE0LfkOkgJ
JJisdnFpBuGzrzcWSgzPiNtn1jh246IlfHEbhmGWp5pZokx4nxkxiprrcBEc7XN7
XNHgAQKBgQCgiC8H2bpqmLii5xp0ZyyzUYn6sOar0nmKNcwpoPPIaOXNyIk7Ildn
3bocdUVFL/oYJFav2cmfeLOaetSEqYzMZO2OzF+OHT3wLBIhUFKgHIuStHUScCAR
FHsqaZxQEdI187t9xlVPGRYTMnKYdvM8pZXe3VFMrNOwRM01xoqSvw==
-----END RSA PRIVATE KEY-----`
)

func makeSecret(secretName string) cache.Resource {
	// Load CA
	catls, err := tls.X509KeyPair([]byte(caCert), []byte(caKey))
	if err != nil {
		panic(err)
	}
	ca, err := x509.ParseCertificate([]byte(caCert))
	if err != nil {
		panic(err)
	}

	// Prepare certificate
	template := &x509.Certificate{
		Subject: pkix.Name{
			Organization: []string{"Shopify"},
			Country:      []string{"CA"},
			Province:     []string{"ON"},
			Locality:     []string{"Ottawa"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pub := &priv.PublicKey

	// Sign the certificate
	certRaw, err := x509.CreateCertificate(rand.Reader, template, ca, pub, catls.PrivateKey)

	var cert, key bytes.Buffer
	pem.Encode(cert, &pem.Block{Type: "CERTIFICATE", Bytes: certRaw})
	pem.Encode(key, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return []cache.Resource{
		&auth.Secret{
			Name: secretName,
			Type: &auth.Secret_TlsCertificate{
				TlsCertificate: &auth.TlsCertificate{
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: key.Bytes()},
					},
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: cert.Bytes()},
					},
				},
			},
		},
		&auth.Secret{
			Name: secretName + "_validation",
			Type: &auth.Secret_ValidationContext{
				ValidationContext: &auth.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(caCert)},
					},
					VerifyCertificateHash: []string{app1Hash},
				},
			},
		},
	}
}

func makeSecrets() []cache.Resource {
	return []cache.Resource{
		&auth.Secret{
			Name: "app1_cert",
			Type: &auth.Secret_TlsCertificate{
				TlsCertificate: &auth.TlsCertificate{
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(app1Key)},
					},
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(app1Cert)},
					},
				},
			},
		},
		&auth.Secret{
			Name: "app2_cert",
			Type: &auth.Secret_TlsCertificate{
				TlsCertificate: &auth.TlsCertificate{
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(app2Key)},
					},
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(app2Cert)},
					},
				},
			},
		},
		&auth.Secret{
			Name: "app_ca",
			Type: &auth.Secret_ValidationContext{
				ValidationContext: &auth.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(caCert)},
					},
				},
			},
		},
		&auth.Secret{
			Name: "server_cert",
			Type: &auth.Secret_TlsCertificate{
				TlsCertificate: &auth.TlsCertificate{
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(serverKey)},
					},
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(serverCert)},
					},
				},
			},
		},
		&auth.Secret{
			Name: "redis1_validation",
			Type: &auth.Secret_ValidationContext{
				ValidationContext: &auth.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(caCert)},
					},
					VerifyCertificateHash: []string{app1Hash},
				},
			},
		},
		&auth.Secret{
			Name: "redis2_validation",
			Type: &auth.Secret_ValidationContext{
				ValidationContext: &auth.CertificateValidationContext{
					TrustedCa: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{InlineBytes: []byte(caCert)},
					},
					VerifyCertificateHash: []string{app2Hash},
				},
			},
		},
	}
}
