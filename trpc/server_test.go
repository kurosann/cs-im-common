package trpc

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"vane-common/server/test"
	"vane-common/trpc/common"
)

type testServiceServer struct {
	test.UnimplementedTestServiceServer
}

func (s *testServiceServer) Test(ctx context.Context, req *test.TestRequest) (*test.TestResponse, error) {
	token := ctx.Value(common.Token).(string)
	if token != "123456" {
		return nil, status.Errorf(codes.Unauthenticated, "token is invalid")
	}
	return &test.TestResponse{Result: "Hello, World!"}, nil
}

func TestNewServer(t *testing.T) {
	InitTrpcDiscover("test", "localhost:8080", NewSimpleSvcmap(map[string][]string{
		"test": {"localhost:8080"},
	}))
	server := NewSvcServer("test")
	server.RegisterService(&test.TestService_ServiceDesc, &testServiceServer{})
	go func() {
		lis, err := net.Listen("tcp", ":8080")
		if err != nil {
			t.Fatal(err)
		}
		if err := server.Serve(lis); err != nil {
			t.Fatal(err)
		}
	}()

	conn, err := NewTokenClient(SvcName("test"), "123456")
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
