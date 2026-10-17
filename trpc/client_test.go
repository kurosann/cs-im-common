package trpc

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc/credentials/insecure"

	"vane-common/server/test"
	"vane-common/trpc/common"

	"google.golang.org/grpc"
)

func init() {
	InitTrpcDiscover("test", "localhost:8080", NewSimpleSvcmap(map[string][]string{
		"test": {"localhost:8080"},
	}))
}
func TestRequest(t *testing.T) {
	conn, err := NewClient("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := test.NewTestServiceClient(conn)
	resp, err := client.Test(context.Background(), &test.TestRequest{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp)
}

type TestServiceServer struct {
	test.UnimplementedTestServiceServer
}

func (s *TestServiceServer) Test(ctx context.Context, req *test.TestRequest) (*test.TestResponse, error) {
	return &test.TestResponse{Result: "Hello, " + req.Name}, nil
}
func TestDiscover(t *testing.T) {
	go func() {
		server := NewServer()
		test.RegisterTestServiceServer(server, &TestServiceServer{})
		lis, err := net.Listen("tcp", ":8080")
		if err != nil {
			t.Fatal(err)
		}
		server.Serve(lis)
	}()
	time.Sleep(time.Second)

	conn, err := NewInternalClient("test")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := test.NewTestServiceClient(conn)
	resp, err := client.Test(context.Background(), &test.TestRequest{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp)
}

func TestInterceptor(t *testing.T) {
	conn, err := NewClient("localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := test.NewTestServiceClient(conn)
	resp, err := client.Test(context.Background(), &test.TestRequest{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp)
}

func TestExampleCertificateUsage(t *testing.T) {
	cfg := &ClientTLSConfig{
		CertFile:   "client.crt",
		KeyFile:    "client.key",
		CAFile:     "ca.crt",
		ServerName: "myservice.example.com",
	}
	creds, err := NewClientCredentials(cfg)
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	conn, err := NewClient("myservice.example.com:8443", grpc.WithTransportCredentials(creds))
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	conn.Close()

	cfg2 := &ClientTLSConfig{
		CertFile:   "tglink.crt",
		KeyFile:    "tglink.key",
		CAFile:     "ca.crt",
		ServerName: "localhost",
	}
	if err := InitClientMTLSForHost(cfg2); err != nil {
		t.Skipf("skip: %v", err)
	}
	conn2, err := NewTokenClient("test", common.InternalToken)
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	conn2.Close()

	conn3, err := NewTokenClientWithHost("localhost:8080", common.InternalToken)
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	conn3.Close()

	_, err = GetSvcConnWithHost(GetInternalCtx(context.Background()), "localhost:8080")
	if err != nil {
		t.Skipf("skip: %v", err)
	}

	cfgPEM := &ClientTLSPEMConfig{
		CertPEM:    []byte("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"),
		KeyPEM:     []byte("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"),
		CAPEM:      []byte("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"),
		ServerName: "myservice.example.com",
	}
	credsPEM, err := NewClientCredentialsFromPEM(cfgPEM)
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	conn5, err := NewClient("myservice.example.com:8443", grpc.WithTransportCredentials(credsPEM))
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	conn5.Close()
}

func TestExampleServerCertificateUsage(t *testing.T) {
	cfg := &ServerTLSConfig{
		CertFile: "server.crt",
		KeyFile:  "server.key",
		CAFile:   "ca.crt",
	}
	if err := SetServerMTLS(cfg); err != nil {
		t.Skipf("skip: %v", err)
	}
	SetServerCreds(nil)
}
