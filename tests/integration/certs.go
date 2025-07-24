package integration

import (
	"crypto/tls"
	"crypto/x509"
)

// Test certificates for integration testing
// These are self-signed certificates generated for testing purposes only
// DO NOT use these in production

const (
	// CA Certificate
	testCACert = `-----BEGIN CERTIFICATE-----
MIIDQDCCAigCCQD9ureFmpIxSjANBgkqhkiG9w0BAQsFADBiMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNVBAoM
BFRlc3QxDTALBgNVBAsMBFRlc3QxEDAOBgNVBAMMB1Rlc3QgQ0EwHhcNMjUwNzIz
MDE1MzAxWhcNMjYwNzIzMDE1MzAxWjBiMQswCQYDVQQGEwJVUzELMAkGA1UECAwC
Q0ExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNVBAoMBFRlc3QxDTALBgNV
BAsMBFRlc3QxEDAOBgNVBAMMB1Rlc3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQCzeLyE1gDCadoRzYV+uJqlqCSaqJ3HADZwvro+6FQZ82jEZ0W/
nOJbXPcr7lJ4JWO0bAM4Csabv1UFOzELJ1+BFEQQea+H5CB3Q2nyiS55wj7m5xTx
OZv9PBber7wWWHBnhh2iGuGQz3nq+5hVY2evsw67M4zReQgN1MBuF4XH4khUCxIO
mADU370Ynsp+0TMuf/lbviDSiRvSx/N8EJKrRebPT0n1GjGa0VxN4m7gmDzXrHQT
DaLiKM+IoSuLOS88W/6hLjzvQw7Gy/jHs/6E5M+jZ9hybAzB/EjvtyP2QGornyhV
sV9inmUh0fUJ3Lo8F1Few6gAblvYIqZEeudpAgMBAAEwDQYJKoZIhvcNAQELBQAD
ggEBAEbPwFDDU8p+MBZwCn3UfAj8VN/+6VdafvvG/fuwDdeN7nCKQiTBYNmTHjzc
qf6jygoaLmatjmPfBXhkp4LmUxsyIW+SUYtNDJHjhZGkjaU39VjVY8xVvcOpTRQO
FlrNIFR4pDE7eLF9Cpe+lEW1cy3J8VK7zTA6yel4UjWfGlSIVOvE2HBLUb4BmJiB
aNaRaYKH1ZUtA279WmzanG0WB8K6kqntjzInlWXJYiaeGynbFcdsj5PoqrqjT7wr
aoBSn/o4NYSkn3krwJtiktZ28usudQbZqwxfyyOkp2YpvNahCIkGSpr4qGIGsZnw
BKXAOrD36B+qyeq8JpQNf57ZjVo=
-----END CERTIFICATE-----`

	// CA Private Key
	testCAKey = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCzeLyE1gDCadoR
zYV+uJqlqCSaqJ3HADZwvro+6FQZ82jEZ0W/nOJbXPcr7lJ4JWO0bAM4Csabv1UF
OzELJ1+BFEQQea+H5CB3Q2nyiS55wj7m5xTxOZv9PBber7wWWHBnhh2iGuGQz3nq
+5hVY2evsw67M4zReQgN1MBuF4XH4khUCxIOmADU370Ynsp+0TMuf/lbviDSiRvS
x/N8EJKrRebPT0n1GjGa0VxN4m7gmDzXrHQTDaLiKM+IoSuLOS88W/6hLjzvQw7G
y/jHs/6E5M+jZ9hybAzB/EjvtyP2QGornyhVsV9inmUh0fUJ3Lo8F1Few6gAblvY
IqZEeudpAgMBAAECggEABJveI38nZ9a2Dez8N6Pf/M8TmZEo9BpSS5TqTYFD36K7
lweb5+7MdVIu2sb1ATbcl56Kep70OL2yHj9F5CZvsm3lzZKCanf2SXnGt77EHcZa
PS3EAOnc0qT/ZVqX9u7wfAgarLYKAuEBHYK2h8LUv9NWVoJdZGe6SDildG5QNjDT
aJ/xYGQrj3w7c14aNlajZokarEOlff9VzJmm/6isyva6Vjwj6Vryvg6tzUWIo6R8
2pySUvuBuoKgPVEXGrUKQO9Z6miiplVz4ot9vpIFbOP6Se+1IFis17kPuuj5XjfQ
mqIgFursNW962xzJAo/nik9JmtXBr0/LKxtfNEVuDQKBgQDkciMmd63dbK/23cTt
k5V3iHVbFOY2onFC+Zxc2Nn+gX5hseIlUePcuBrVswiAMZIjskertUduJEg4jYzs
zlmyKfM+c0IPMRPZasITehcnqK8A2Apvqa+21jbDNDX91lj8EExFS4GFvU9pgd05
LS8qEhuW+r/cx8rCRFPjpSoG0wKBgQDJHmRYcu8DXUQqrtOJzNWOJDcnk05K+8s3
A4pC7hnC6mw1alIyjmUDqIsLQHKC9qXjvZckAQNl3tTDZ8l+zn7ntFBIqHBaUM1e
be4q9bqnQ25dfGuys3h3UxGCFg31uYGyLvyu3BiO59Ph1AgvaYtSreNnUgLNVyPo
O6d3/a3rUwKBgQDfvJGEiU41QM+OHmFStWp76Z/Wlr9p3urCx6lGnfPS+YyHripo
lq1ubLmLdo7qzqHsaB0dpKvSyaIaEThmbSsX/VIIZeXa7xwboh116etnoiPT1cNS
3YQEtARqZmZCt33rUSMB8xNloqV2FgROjVxV/eobknX6i4qffUAUAp0IlQKBgHOG
NqOr2Wk4UKin5bD47Q6J9PiRn95ohhFiwi+x7zBMUb3ZBcAulQ2l6cCb02sw3JdV
1xSCVH5WoiZgXpitaq4ToC4sOuVWFrGQOceJgR8FF8cxaferKZ55I8xyeLBWT46X
eOPEX4Lu3YGRtuXtHW9vnPlDXYKv9Fs4sPi2ygkrAoGAGv/E1GrNIo7/IXR9b07A
ft8i4kiPDwDm6+Z2akgNw1HtcIUQ4pHIZH7KJH3vNcwUdnW8bGlZQJS//0P+J0nM
Om9i0xyWcs+0wMpVqhyhSIr1nROCG8WnXcBiOVHV3unIOORPrAx+QaSm2190fqzO
+uCH33Uq8y/hAjNGTFj/OSA=
-----END PRIVATE KEY-----`

	// Server Certificate
	testServerCert = `-----BEGIN CERTIFICATE-----
MIIDQjCCAioCCQDOpCPesIfV3TANBgkqhkiG9w0BAQsFADBiMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNVBAoM
BFRlc3QxDTALBgNVBAsMBFRlc3QxEDAOBgNVBAMMB1Rlc3QgQ0EwHhcNMjUwNzIz
MDE1MzI2WhcNMjYwNzIzMDE1MzI2WjBkMQswCQYDVQQGEwJVUzELMAkGA1UECAwC
Q0ExFjAUBgNVBAcMDVNhbiBGcmFuY2lzY28xDTALBgNVBAoMBFRlc3QxDTALBgNV
BAsMBFRlc3QxEjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAMJCknBLjJPGnuhgkt8x/kIp4ogleokRUus+twXeQAVMpsc0
g0jNCS+vDYAKVrM/gEdx7SkbLHCXNsffHFBwATH8IaDZ1YHuRIZWS9oNsgszM6xd
xzFtmKZrTjXlvnuwEJG/qG9R7n5nHLP21kFqAJYEhSu6ckFMkyvtXBrZGZecg9Ta
lniuC/DWZQ6I+B1EoBWRW1gccTqR6G/e3smLz09KcYlIm+EeZGHDmh4qC3FgEr6C
Wp0d4jUeSQv/ps4z9PALkB8QsefFVh+UWfhZlRWLYDMgBleFGBJvngzq63J2AUg8
N6Q4p9jcT6ZEpKFJ5nxhj23R7tWfzYZffWfVBKsCAwEAATANBgkqhkiG9w0BAQsF
AAOCAQEAbYQ6fnAo0BfeVlEhBAn1iLjguhmCKoeF9lt0BXf9J5pSAE53/FNpEg6X
XvKp/+f+fkgLWlmmhmmzCmFP9nRczD8DUWTkwFfw0d3LmcyeDrEN8KFyJaxchh1t
mq1Eh9VkYGTMxysVJ7VLHtUHF+JABClbuGPMsspLBRLCtkkFMq67TP1S5/bq//Oz
gX6oqUacdFePcqLKeSRpOpZESW407/tKi9J7XZAU+SHGgoXcJSQwYq9bsIiK9JFL
hTJQW7YFad7v70wWh5BV8ZSwPiMJOZuAm83njRkDKnQs4fjkzhf2a9L9mOWtp21b
eQdZ/Z7vAiJ3xTuja27bxsxzmp4fJg==
-----END CERTIFICATE-----`

	// Server Private Key
	testServerKey = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDCQpJwS4yTxp7o
YJLfMf5CKeKIJXqJEVLrPrcF3kAFTKbHNINIzQkvrw2AClazP4BHce0pGyxwlzbH
3xxQcAEx/CGg2dWB7kSGVkvaDbILMzOsXccxbZima0415b57sBCRv6hvUe5+Zxyz
9tZBagCWBIUrunJBTJMr7Vwa2RmXnIPU2pZ4rgvw1mUOiPgdRKAVkVtYHHE6kehv
3t7Ji89PSnGJSJvhHmRhw5oeKgtxYBK+glqdHeI1HkkL/6bOM/TwC5AfELHnxVYf
lFn4WZUVi2AzIAZXhRgSb54M6utydgFIPDekOKfY3E+mRKShSeZ8YY9t0e7Vn82G
X31n1QSrAgMBAAECggEAbNP0y/pXH/aW0aJAxc9xHMnwQcuVUTKmXGn/CMeQ4Cco
C9OMdP2A1vjfvEqOdc7uY5gcf/ncNJtSMjj42MtWsBULFdzTcv3z37p6tgcUJpgh
q7/Btxwp95mH8EPsKcjiD3TqvKqOzLuhZeSz9WOYPnL71BqYpaJrlKFeByB26Oro
68ntQH7e+0B7kjWFR4d+rVZ0wg15hwMBXNupCGPQnG2WSg0h4qWY/0MDtqsJOJ0l
XXnzbh2jiXUyDZdXCfJrH+BnNK5xR609rp85lHlUrZ7i/nNmrZ3QJPa7tf3G5x7b
vmSlTBB/JtMuaREs1DI7oFHKHCLIYR3baY9bTHWy8QKBgQDrNjdsp+B7yE4rySjW
zw+lXnNuemjZmoAd7WHCjYes2XkDw9NmbPUzPZkTKK5NjPxfVJNnBIZQrnDF4+Mm
tVZ811otj5MFaRV4yi9uVATzdqVz1XHwdXZanQg/GVKJgAIJK5SmHt8lwPu4n2qX
Tgb5XTauG7LRwImpY7+r8/ROqQKBgQDTbc1Y8pkzzE1MnX6zoC+KDYG+pdz081Nv
i7hmOHtqU01cu5ToJI7a97HS/2Spw72162t7uqq0Klkeevw4PgFO1JSi465NGf3b
noffcuqMeRpBbDOFDfJkGx+BnUjf3dDRUIHc1ZI3DTa9+WBmvyCmalt/Tp10Oho+
jTdpOiMxMwKBgHBAvrDHabYJgW0aIrhpt3DXo8VM/C8lshEWUjqUavTOERf/5CsU
wuzCcASZvJ3cNDGW3oYivatRpRZ8TNMTZgRMjogB5kuFvC6aZ4qC5J4AuLOQYUE9
/c7+9ImQnzhp9A7GUrn5L8wHztpskmVFYsStfMQZCf1aoxhJN5dr5OOJAoGBAL+X
dMnxrRrfO/z9i19C/VFgw/37V6sxBJ7EQil/bXcAXc52vY1P85RBeQb3IEUmd7du
ykuo8B+rcG7Ki9x7c7v3r2mcYMrFjuGBWycFf74jz8MRRe6AoPJOEdLmsK8M1rmW
9tcjQghZFQ45+T2iXPfw0VEf8Fbuf/HHDjtwz4s5AoGAG6qn0Mt5DFGmiv8hqfKp
DY0d5X1W1iMUja1SFjCNX96weIXLK9HSuUjiWziJLsynANHouHdsBS8dpwmduGrh
+21FUhYpA1fo/YCxETm0zQn9tM0eGb7yOrTblMGn//AnN1oebM4UL58EH+MB/KLy
w+lCCAvMUTKVKpB5TF1Kyfk=
-----END PRIVATE KEY-----`
)

// getTestTLSConfig returns a TLS configuration for testing with embedded certificates
func getTestTLSConfig() *tls.Config {
	// Load server certificate and key
	serverCert, err := tls.X509KeyPair([]byte(testServerCert), []byte(testServerKey))
	if err != nil {
		panic("Failed to load server certificate: " + err.Error())
	}

	// Load CA certificate
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM([]byte(testCACert)) {
		panic("Failed to load CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caCertPool,
		RootCAs:      caCertPool,
		ServerName:   "localhost",
		// For testing purposes, we'll be more permissive
		InsecureSkipVerify: false,
		ClientAuth:         tls.NoClientCert, // Don't require client certificates for testing
		// Allow legacy Common Name usage for testing
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return nil // Accept all certificates for testing
		},
	}
}

// getTestClientTLSConfig returns a TLS configuration for test clients
func getTestClientTLSConfig() *tls.Config {
	// Load CA certificate for client to verify server
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM([]byte(testCACert)) {
		panic("Failed to load CA certificate for client")
	}

	return &tls.Config{
		RootCAs:    caCertPool,
		ServerName: "localhost",
		// For testing, skip certificate verification to avoid SAN issues
		InsecureSkipVerify: true,
	}
}
