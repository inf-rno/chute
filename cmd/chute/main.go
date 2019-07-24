package main

import (
	"context"
	"log"
	"os"
	"sync"

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
	app1Key = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAqLi5RmzH6TleUof821DaOhh84aQdUP1Ksc5HZoHReLc5t8PN
0KpGtv0OEeP14t4Jm1JbM38NgMfKLiGz5ceZv9f8rb1FTnMr5LLvmnvdlj+tp7pC
90Sy7eHLb0ogNIvRXhazZrjPkf3EnTH/bZdtIXi9v24ApiJTa+ovijLGUJVptxkZ
2q34fEVsMs6lQyy07zjvofY8R3ympzdQcCJtEWB0YMX8noXpeGr+rfcbMpROHbRr
5OAzu2jvfCHzbZ+qx8riGNoPFqeDMG7PWfrl0gnPPzUwjpvRkrxtoSUjCHanZxEy
3+Xk4kVAdxReHpd51zYTkxa9IgMnS+2yGKpKawIDAQABAoIBAA1Ivg21cugCBFMr
MdVywDviwbJiYYyG5OKrAyQnBH8krf6yA/px7a9qrTjrYejC4q7ABT5AuqdxE5Ie
RTPKS2i3cMWdKV/L4aDYFdVr+z5hNSMHn04oso3YQVQ52d9JQurNjsJ/upgcCub1
kM7oJUeFYis4VgS+nyLYBXY0GTku6aheM/+Z0xTEvIc3Cfzb94aCqnGwHe+TvxN0
zCKhm4bUO4HE5UQzHXO9oxMlOlI7hDAUTSfhT0FpOtxnmisl+XFRSGSvJmNbK4jJ
8dlqIuXsSh4X9NPEnhPz1ieg4+hcUelg5SvSNLNAVQPmm1nNluwYhT41iU+seiiJ
A3uUJEECgYEA2fAS32IQox/8+OpKPipvR13tHek04AsJSGuNiio8qXlQorlXBRYE
esOEEYJyKiNWZ7NXTCAjyDiRp48aM62Fy1ykDR4IbDb8JRAdZpJBCjy5U5BKfZTY
jR3rZOA+8Bivozmtdv+0RGxhffSIGPpB1/4Dgmkw375YLAB4cVffg30CgYEAxjA1
o8ImIW+9eKTRV2yWUHiUZ4iae5s54nCnz24YqJVLooWtU/TXFKrTiYJs20HBWiG0
+dsJ27LXd0Y13DDQN/ZT/luvWbhMkFYGQ7Ou7XDXB6ujgl6A5MZ7mKfxIv+kX5QM
F1NTVl3TeUU+6XAYeUf1xmlDjot3sNNv5NdvGgcCgYBRzUnYLP/fqscSSyaY1Oa1
2+x/mKQvIBVY6H3VCWuBlTaODZE7KHt/9NkilVrytBbfj7JJsZqcsZcCVLVaBly8
60XsYoR40d6srrLKaEUfaZGKaxN6tZ7ewQc08vLMvgdW9fRFQU9Ri3jAhUN8VJrY
TtDUZ1Vf9hs0UOzkZj5QJQKBgCfS7iRe0eysGGWSsOIhVr8Ky79WKrylv2bp/j5n
QBs4DL+2ntKdA08K2IDsLVWNi/3Bgi0mv39fG37DI/V/9YcZP12ALOcZaoEiWBXo
mEDsCLlo2u1KchoGbDWLoZ/HwM7X3+ob+0YCioj2yiJ8PN66AAADjOiqy71Db1uL
kq6nAoGAN8dAwrJFQ2mQYKFQrbWt3/x+YHCAvkqIFdVWn9+3/fWZ8Rcwctl/mau/
7dL5tU/s3wO8KkOH2PTjvtUORZR+e/FGq7hmXVV1lnl7GBENW6THN08xq9e/4Clm
koYsIpn/e+t2AVR1UENvv9sKwAsDV1yqHwRfUSFtg/qtj7103YY=
-----END RSA PRIVATE KEY-----`

	app1Cert = `-----BEGIN CERTIFICATE-----
MIIEZDCCA0ygAwIBAgIJAILStmLgUUcXMA0GCSqGSIb3DQEBCwUAMHYxCzAJBgNV
BAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1TYW4gRnJhbmNp
c2NvMQ0wCwYDVQQKDARMeWZ0MRkwFwYDVQQLDBBMeWZ0IEVuZ2luZWVyaW5nMRAw
DgYDVQQDDAdUZXN0IENBMB4XDTE4MTIxNzIwMTgwMFoXDTIwMTIxNjIwMTgwMFow
gagxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1T
YW4gRnJhbmNpc2NvMQ0wCwYDVQQKDARMeWZ0MRkwFwYDVQQLDBBMeWZ0IEVuZ2lu
ZWVyaW5nMRswGQYDVQQDDBJUZXN0IEZyb250ZW5kIFRlYW0xJTAjBgkqhkiG9w0B
CQEWFmZyb250ZW5kLXRlYW1AbHlmdC5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQCouLlGbMfpOV5Sh/zbUNo6GHzhpB1Q/UqxzkdmgdF4tzm3w83Q
qka2/Q4R4/Xi3gmbUlszfw2Ax8ouIbPlx5m/1/ytvUVOcyvksu+ae92WP62nukL3
RLLt4ctvSiA0i9FeFrNmuM+R/cSdMf9tl20heL2/bgCmIlNr6i+KMsZQlWm3GRna
rfh8RWwyzqVDLLTvOO+h9jxHfKanN1BwIm0RYHRgxfyehel4av6t9xsylE4dtGvk
4DO7aO98IfNtn6rHyuIY2g8Wp4Mwbs9Z+uXSCc8/NTCOm9GSvG2hJSMIdqdnETLf
5eTiRUB3FF4el3nXNhOTFr0iAydL7bIYqkprAgMBAAGjgcEwgb4wDAYDVR0TAQH/
BAIwADALBgNVHQ8EBAMCBeAwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMB
MEIGA1UdEQQ7MDmGH3NwaWZmZTovL2x5ZnQuY29tL2Zyb250ZW5kLXRlYW2CCGx5
ZnQuY29tggx3d3cubHlmdC5jb20wHQYDVR0OBBYEFJD5l1K7mp/dUUNXvXPBWJ5l
oJOnMB8GA1UdIwQYMBaAFBQz1vaSbPuePL++7GTMqLAMtk3kMA0GCSqGSIb3DQEB
CwUAA4IBAQCJOUQB5d8ZCEeMrY0jLwefY8L0UluhPFWNlw1t2LyDjAa8qRNkWJ/2
bky7IHBMxPQIdYBVPsQGOQ4bkg3S7Eqyc0WZYpLlKEQeUFPG752642GInzjgY3KG
f1hIHz1quIYARjF5GJ+buZpw3DgcGDnhYygFQDWqgyRnfz84M1ycEx06yHidupyp
eMHZHXcrSXPcGin7a6tBEppDFm5CcrJQ2hySDVkl9qnbgHr0+0JZg/Qekik4aWv5
ACWk3wTxIPUv6mc8kbBMRMPkETzWt4m/qdLnUUhFKJdyACPlb6onJe1TGuYJvc2x
rNV3U8Yo8a1iskQtHqNfc+kqVd3MpgsB
-----END CERTIFICATE-----`

	app1Hash = `90:CA:A3:E0:B0:AD:8E:E6:4F:BC:11:6C:7B:E5:9D:35:11:2B:46:71:5F:4D:5C:52:85:37:23:08:38:28:B4:D6`

	app2Key = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAqLi5RmzH6TleUof821DaOhh84aQdUP1Ksc5HZoHReLc5t8PN
0KpGtv0OEeP14t4Jm1JbM38NgMfKLiGz5ceZv9f8rb1FTnMr5LLvmnvdlj+tp7pC
90Sy7eHLb0ogNIvRXhazZrjPkf3EnTH/bZdtIXi9v24ApiJTa+ovijLGUJVptxkZ
2q34fEVsMs6lQyy07zjvofY8R3ympzdQcCJtEWB0YMX8noXpeGr+rfcbMpROHbRr
5OAzu2jvfCHzbZ+qx8riGNoPFqeDMG7PWfrl0gnPPzUwjpvRkrxtoSUjCHanZxEy
3+Xk4kVAdxReHpd51zYTkxa9IgMnS+2yGKpKawIDAQABAoIBAA1Ivg21cugCBFMr
MdVywDviwbJiYYyG5OKrAyQnBH8krf6yA/px7a9qrTjrYejC4q7ABT5AuqdxE5Ie
RTPKS2i3cMWdKV/L4aDYFdVr+z5hNSMHn04oso3YQVQ52d9JQurNjsJ/upgcCub1
kM7oJUeFYis4VgS+nyLYBXY0GTku6aheM/+Z0xTEvIc3Cfzb94aCqnGwHe+TvxN0
zCKhm4bUO4HE5UQzHXO9oxMlOlI7hDAUTSfhT0FpOtxnmisl+XFRSGSvJmNbK4jJ
8dlqIuXsSh4X9NPEnhPz1ieg4+hcUelg5SvSNLNAVQPmm1nNluwYhT41iU+seiiJ
A3uUJEECgYEA2fAS32IQox/8+OpKPipvR13tHek04AsJSGuNiio8qXlQorlXBRYE
esOEEYJyKiNWZ7NXTCAjyDiRp48aM62Fy1ykDR4IbDb8JRAdZpJBCjy5U5BKfZTY
jR3rZOA+8Bivozmtdv+0RGxhffSIGPpB1/4Dgmkw375YLAB4cVffg30CgYEAxjA1
o8ImIW+9eKTRV2yWUHiUZ4iae5s54nCnz24YqJVLooWtU/TXFKrTiYJs20HBWiG0
+dsJ27LXd0Y13DDQN/ZT/luvWbhMkFYGQ7Ou7XDXB6ujgl6A5MZ7mKfxIv+kX5QM
F1NTVl3TeUU+6XAYeUf1xmlDjot3sNNv5NdvGgcCgYBRzUnYLP/fqscSSyaY1Oa1
2+x/mKQvIBVY6H3VCWuBlTaODZE7KHt/9NkilVrytBbfj7JJsZqcsZcCVLVaBly8
60XsYoR40d6srrLKaEUfaZGKaxN6tZ7ewQc08vLMvgdW9fRFQU9Ri3jAhUN8VJrY
TtDUZ1Vf9hs0UOzkZj5QJQKBgCfS7iRe0eysGGWSsOIhVr8Ky79WKrylv2bp/j5n
QBs4DL+2ntKdA08K2IDsLVWNi/3Bgi0mv39fG37DI/V/9YcZP12ALOcZaoEiWBXo
mEDsCLlo2u1KchoGbDWLoZ/HwM7X3+ob+0YCioj2yiJ8PN66AAADjOiqy71Db1uL
kq6nAoGAN8dAwrJFQ2mQYKFQrbWt3/x+YHCAvkqIFdVWn9+3/fWZ8Rcwctl/mau/
7dL5tU/s3wO8KkOH2PTjvtUORZR+e/FGq7hmXVV1lnl7GBENW6THN08xq9e/4Clm
koYsIpn/e+t2AVR1UENvv9sKwAsDV1yqHwRfUSFtg/qtj7103YY=
-----END RSA PRIVATE KEY-----`

	app2Cert = `-----BEGIN CERTIFICATE-----
MIIEZDCCA0ygAwIBAgIJAILStmLgUUcXMA0GCSqGSIb3DQEBCwUAMHYxCzAJBgNV
BAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1TYW4gRnJhbmNp
c2NvMQ0wCwYDVQQKDARMeWZ0MRkwFwYDVQQLDBBMeWZ0IEVuZ2luZWVyaW5nMRAw
DgYDVQQDDAdUZXN0IENBMB4XDTE4MTIxNzIwMTgwMFoXDTIwMTIxNjIwMTgwMFow
gagxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1T
YW4gRnJhbmNpc2NvMQ0wCwYDVQQKDARMeWZ0MRkwFwYDVQQLDBBMeWZ0IEVuZ2lu
ZWVyaW5nMRswGQYDVQQDDBJUZXN0IEZyb250ZW5kIFRlYW0xJTAjBgkqhkiG9w0B
CQEWFmZyb250ZW5kLXRlYW1AbHlmdC5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQCouLlGbMfpOV5Sh/zbUNo6GHzhpB1Q/UqxzkdmgdF4tzm3w83Q
qka2/Q4R4/Xi3gmbUlszfw2Ax8ouIbPlx5m/1/ytvUVOcyvksu+ae92WP62nukL3
RLLt4ctvSiA0i9FeFrNmuM+R/cSdMf9tl20heL2/bgCmIlNr6i+KMsZQlWm3GRna
rfh8RWwyzqVDLLTvOO+h9jxHfKanN1BwIm0RYHRgxfyehel4av6t9xsylE4dtGvk
4DO7aO98IfNtn6rHyuIY2g8Wp4Mwbs9Z+uXSCc8/NTCOm9GSvG2hJSMIdqdnETLf
5eTiRUB3FF4el3nXNhOTFr0iAydL7bIYqkprAgMBAAGjgcEwgb4wDAYDVR0TAQH/
BAIwADALBgNVHQ8EBAMCBeAwHQYDVR0lBBYwFAYIKwYBBQUHAwIGCCsGAQUFBwMB
MEIGA1UdEQQ7MDmGH3NwaWZmZTovL2x5ZnQuY29tL2Zyb250ZW5kLXRlYW2CCGx5
ZnQuY29tggx3d3cubHlmdC5jb20wHQYDVR0OBBYEFJD5l1K7mp/dUUNXvXPBWJ5l
oJOnMB8GA1UdIwQYMBaAFBQz1vaSbPuePL++7GTMqLAMtk3kMA0GCSqGSIb3DQEB
CwUAA4IBAQCJOUQB5d8ZCEeMrY0jLwefY8L0UluhPFWNlw1t2LyDjAa8qRNkWJ/2
bky7IHBMxPQIdYBVPsQGOQ4bkg3S7Eqyc0WZYpLlKEQeUFPG752642GInzjgY3KG
f1hIHz1quIYARjF5GJ+buZpw3DgcGDnhYygFQDWqgyRnfz84M1ycEx06yHidupyp
eMHZHXcrSXPcGin7a6tBEppDFm5CcrJQ2hySDVkl9qnbgHr0+0JZg/Qekik4aWv5
ACWk3wTxIPUv6mc8kbBMRMPkETzWt4m/qdLnUUhFKJdyACPlb6onJe1TGuYJvc2x
rNV3U8Yo8a1iskQtHqNfc+kqVd3MpgsB
-----END CERTIFICATE-----`

	app2Hash = `90:CA:A3:E0:B0:AD:8E:E6:4F:BC:11:6C:7B:E5:9D:35:11:2B:46:71:5F:4D:5C:52:85:37:23:08:38:28:B4:D6`

	serverKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAuvPdQdmwZongPAgQho/Vipd3PZWrQ6BKxIb4l/RvqtVP321I
UTLs4vVwpXoYJ+12L+XOO3jCInszs53tHjFpTI1GE8/sasmgR6LRr2krwSoVRHPq
Uoc9tzkDG1SzKP2TRTi1MTI3FO+TnLFahntO9Zstxhv1Epz5GZ/xQLE0/LLoRYzc
ynL/iflk18iL1KM8i0Hy4cKjclOaUdnh2nh753iJfxCSb5wJfx4FH1qverYHHT6F
opYRV40Cg0yYXcYo8yNwrg+EBY8QAT2JOMDokXNKbZpmVKiBlh0QYMX6BBiW249v
3sYl3Ve+fZvCkle3W0xP0xJw8PdX0NRbvGOrBQIDAQABAoIBAQCkPLR1sy47BokN
c/BApn9sn5/LZH7ujBTjDce6hqzLIVZn6/OKEfj1cbWiSd6KxRv8/B/vMykpbZ5/
/w9eZP4imEGmChWhwruh8zHOrdAYhEXmuwZxtgnLurQ2AHTcX9hPCYB0Va76H3ZI
Q65JUm6NaeQOlGT6ExjrIA2rTYJFM84I1xH3XbDulS9S2FXNP9RIjV70HzvZw2LR
1qSNfrnGAEbUCdrZT4BAYTGam5L061ofencYLAorr8K0eVWhUjGV9Jjpq8aG8zy5
Oy1070I0d7Iexfu7T1sQDIqpNkOtQxI8feQEKeKlRKYx6YEQ9vaVwBGa0SBVxQem
E3YdXBnBAoGBAORlz8wlYqCx25htO/eLgr9hN+eKNhNTo4l905aZrG8SPinaHl15
n+dQdzlJMVm/rh5+VE0NR0U/vzd3SrdnzczksuGFn0Us/Yg+zOl1+8+GFAtqw3js
udFLKksChz4Rk/fZo2djtSiFS5aGBtw0Z9T7eorubkTSSfJ7IT99HIu5AoGBANGL
0ff5U2LV/Y/opKP7xOlxSCVI617N5i0sYMJ9EUaWzvquidzM46T4fwlAeIvAtks7
ACO1cRPuWredZ/gEZ3RguZMxs6llwxwVCaQk/2vbOfATWmyqpGC9UBS/TpYVXbL5
WUMsdBs4DdAFz8aCrrFBcDeCg4V4w+gHYkFV+LetAoGAB3Ny1fwaPZfPzCc0H51D
hK7NPhZ6MSM3YJLkRjN5Np5nvMHK383J86fiW9IRdBYWvhPs+B6Ixq+Ps2WG4HjY
c+i6FTVgvsb69mjmEm+w6VI8cSroeZdvcG59ULkiZFn6c8l71TGhhVLj5mM08hYb
lQ0nMEUa/8/Ebc6qhQG13rECgYEAm8AZaP9hA22a8oQxG9HfIsSYo1331JemJp19
rhHX7WfaoGlq/zsrWUt64R2SfA3ZcUGBcQlD61SXCTNuO+LKIq5iQQ4IRDjnNNBO
QjtdvoVMIy2/YFXVqDIOe91WRCfNZWIA/vTjt/eKDLzFGv+3aPkCt7/CkkqZErWq
SnXkUGECgYAvkemYu01V1WcJotvLKkVG68jwjMq7jURpbn8oQVlFR8zEh+2UipLB
OmrNZjmdrhQe+4rzs9XCLE/EZsn7SsygwMyVhgCYzWc/SswADq7Wdbigpmrs+grW
fg7yxbPGinTyraMd0x3Ty924LLscoJMWUBl7qGeQ2iUdnELmZgLN2Q==
-----END RSA PRIVATE KEY-----`

	serverCert = `-----BEGIN CERTIFICATE-----
MIIEYTCCA0mgAwIBAgIJAILStmLgUUcVMA0GCSqGSIb3DQEBCwUAMHYxCzAJBgNV
BAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1TYW4gRnJhbmNp
c2NvMQ0wCwYDVQQKDARMeWZ0MRkwFwYDVQQLDBBMeWZ0IEVuZ2luZWVyaW5nMRAw
DgYDVQQDDAdUZXN0IENBMB4XDTE4MTIxNzIwMTgwMFoXDTIwMTIxNjIwMTgwMFow
gaYxCzAJBgNVBAYTAlVTMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1T
YW4gRnJhbmNpc2NvMQ0wCwYDVQQKDARMeWZ0MRkwFwYDVQQLDBBMeWZ0IEVuZ2lu
ZWVyaW5nMRowGAYDVQQDDBFUZXN0IEJhY2tlbmQgVGVhbTEkMCIGCSqGSIb3DQEJ
ARYVYmFja2VuZC10ZWFtQGx5ZnQuY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEAuvPdQdmwZongPAgQho/Vipd3PZWrQ6BKxIb4l/RvqtVP321IUTLs
4vVwpXoYJ+12L+XOO3jCInszs53tHjFpTI1GE8/sasmgR6LRr2krwSoVRHPqUoc9
tzkDG1SzKP2TRTi1MTI3FO+TnLFahntO9Zstxhv1Epz5GZ/xQLE0/LLoRYzcynL/
iflk18iL1KM8i0Hy4cKjclOaUdnh2nh753iJfxCSb5wJfx4FH1qverYHHT6FopYR
V40Cg0yYXcYo8yNwrg+EBY8QAT2JOMDokXNKbZpmVKiBlh0QYMX6BBiW249v3sYl
3Ve+fZvCkle3W0xP0xJw8PdX0NRbvGOrBQIDAQABo4HAMIG9MAwGA1UdEwEB/wQC
MAAwCwYDVR0PBAQDAgXgMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATBB
BgNVHREEOjA4hh5zcGlmZmU6Ly9seWZ0LmNvbS9iYWNrZW5kLXRlYW2CCGx5ZnQu
Y29tggx3d3cubHlmdC5jb20wHQYDVR0OBBYEFLHmMm0DV9jCHJSWVRwyPYpBw62r
MB8GA1UdIwQYMBaAFBQz1vaSbPuePL++7GTMqLAMtk3kMA0GCSqGSIb3DQEBCwUA
A4IBAQAwx3/M2o00W8GlQ3OT4y/hQGb5K2aytxx8QeSmJaaZTJbvaHhe0x3/fLgq
uWrW3WEWFtwasilySjOrFOtB9UNmJmNOHSJD3Bslbv5htRaWnoFPCXdwZtVMdoTq
IHIQqLoos/xj3kVD5sJSYySrveMeKaeUILTkb5ZubSivye1X2yiJLR7AtuwuiMio
CdIOqhn6xJqYhT7z0IhdKpLNPk4w1tBZSKOXqzrXS4uoJgTC67hWslWWZ2VC6IvZ
FmKuuGZamCCj6F1QF2IjMVM8evl84hEnN0ajdkA/QWnil9kcWvBm15Ho+oTvvJ7s
M8MD3RDSq/90FSiME4vbyNEyTmj0
-----END CERTIFICATE-----`

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
)

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
