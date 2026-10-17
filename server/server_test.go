package server

import (
	"context"
	"strconv"
	"testing"
	"time"

	"vane-common/module"
	"vane-common/server/serverconfig"
	"vane-common/server/test"
	"vane-common/trpc"

	"github.com/stretchr/testify/assert"
)

type TestServiceServer struct {
	test.UnimplementedTestServiceServer
}

func (s *TestServiceServer) Test(ctx context.Context, req *test.TestRequest) (*test.TestResponse, error) {
	return &test.TestResponse{Result: "Hello, " + req.Name}, nil
}

var _ module.IModule = (*TestModule)(nil)

type TestModule struct {
	initCalled   bool
	apiCalled    bool
	jobCalled    bool
	jobRunCalled bool
}

func (m *TestModule) Model() []module.ModelRegInfo {
	return []module.ModelRegInfo{}
}

func (m *TestModule) Init(ctx context.Context) error {
	m.initCalled = true
	return nil
}

func (m *TestModule) Api() []module.ApiRegInfo {
	m.apiCalled = true
	return []module.ApiRegInfo{
		module.NewApiRegInfo(test.TestService_ServiceDesc, &TestServiceServer{}),
	}
}

type TestJobTask struct {
	runCalled *bool
}

func (j *TestJobTask) Run(ctx context.Context) {
	*j.runCalled = true
}

func (m *TestModule) Job() []module.JobRegInfo {
	m.jobCalled = true
	return []module.JobRegInfo{
		module.NewJobRegInfo("test-job", "0 0 * * *", &TestJobTask{runCalled: &m.jobRunCalled}),
	}
}

func (m *TestModule) Task() []module.TaskRegInfo {
	return []module.TaskRegInfo{}
}

func TestNewServer(t *testing.T) {
	server := NewServer("test-server")
	assert.NotNil(t, server)
	assert.Equal(t, "test-server", server.name)
}

func TestRegModule(t *testing.T) {
	server := NewServer("test-server")
	testModule := &TestModule{}
	server.WithConfig(func() serverconfig.Config {
		return serverconfig.Config{Port: 8080}
	})

	modules := []module.ModuleInfo{
		{
			Name:   "test-module",
			Module: testModule,
		},
	}

	server = server.RegModule(modules)
	err := server.Init(context.Background())

	assert.NotNil(t, server.moduleManager)
	server.Start(context.Background())
	time.Sleep(10 * time.Millisecond)
	server.Stop()
	if !assert.NoError(t, err) {
		return
	}
	assert.True(t, testModule.initCalled)
	assert.True(t, testModule.apiCalled)
	assert.True(t, testModule.jobCalled)
}

var _ IServer = (*TestExtandServer)(nil)
var _ IDBServer = (*TestDBServer)(nil)

type TestDBServer struct {
	TestExtandServer
	initDBCalled bool
	order        *[]string
}

func (s *TestDBServer) Init(ctx context.Context) error {
	if s.order != nil {
		*s.order = append(*s.order, "db-init")
	}
	return s.TestExtandServer.Init(ctx)
}

func (s *TestDBServer) InitDB(ctx context.Context, regModels ...module.ModelRegInfo) error {
	s.initDBCalled = true
	if s.order != nil {
		*s.order = append(*s.order, "db-initdb")
	}
	return nil
}

type orderModule struct {
	TestModule
	order *[]string
}

func (m *orderModule) Init(ctx context.Context) error {
	if m.order != nil {
		*m.order = append(*m.order, "module-init")
	}
	return m.TestModule.Init(ctx)
}

type TestExtandServer struct {
	initCalled   bool
	runCalled    bool
	stopCalled   bool
	reloadCalled bool
}

func (s *TestExtandServer) Reload(ctx context.Context) error {
	s.reloadCalled = true
	return nil
}

func (s *TestExtandServer) Init(ctx context.Context) error {
	s.initCalled = true
	return nil
}

func (s *TestExtandServer) Start(ctx context.Context) error {
	s.runCalled = true
	return nil
}

func (s *TestExtandServer) Stop() {
	s.stopCalled = true
}

// 测试扩展服务是否正常
func TestRegExtandServer(t *testing.T) {
	server := NewServer("test-server")
	extendServer := &TestExtandServer{}

	server = server.RegExtendServer(extendServer)
	assert.Len(t, server.extandServers, 1)
	// 测试扩展服务的init run stop是否正常
	err := server.Init(context.Background())
	if !assert.NoError(t, err) {
		return
	}
	server.Start(context.Background())
	time.Sleep(10 * time.Millisecond)
	server.Stop()
	assert.True(t, extendServer.initCalled)
	assert.True(t, extendServer.runCalled)
	assert.True(t, extendServer.stopCalled)
}

func TestInitDBServerBeforeModules(t *testing.T) {
	order := make([]string, 0, 3)
	dbServer := &TestDBServer{order: &order}
	mod := &orderModule{order: &order}
	extend := &TestExtandServer{}

	server := NewServer("test-server")
	server = server.WithConfig(func() serverconfig.Config {
		return serverconfig.Config{Port: 18080, Standalone: true, DisableGRPC: true, DisableMonitor: true}
	})
	server = server.RegExtendServer(dbServer, extend)
	server = server.RegModule([]module.ModuleInfo{{Name: "order-module", Module: mod}})

	err := server.Init(context.Background())
	if !assert.NoError(t, err) {
		return
	}
	server.Stop()

	assert.True(t, dbServer.initCalled)
	assert.True(t, dbServer.initDBCalled)
	assert.True(t, mod.initCalled)
	assert.True(t, extend.initCalled)
	assert.Equal(t, []string{"db-init", "db-initdb", "module-init"}, order)
}

// 测试reload是否正常
func TestWithReload(t *testing.T) {
	server := NewServer("test-server")
	reloadCalled := false

	server = server.WithConfig(func() serverconfig.Config {
		return serverconfig.Config{
			Reload: func() error {
				reloadCalled = true
				return nil
			},
		}
	})

	server.Init(context.Background())
	err := server.Reload(context.Background())
	if !assert.NoError(t, err) {
		return
	}
	assert.True(t, reloadCalled)
}

// 测试start和stop是否正常
func TestStartAndStop(t *testing.T) {
	server := NewServer("test-server")
	testModule := &TestModule{}

	modules := []module.ModuleInfo{
		{
			Name:   "test-module",
			Module: testModule,
		},
	}

	server = server.RegModule(modules)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := server.Init(ctx)
	if !assert.NoError(t, err) {
		return
	}

	err = server.Start(ctx)
	if !assert.NoError(t, err) {
		return
	}

	// 等待一小段时间确保服务启动
	time.Sleep(10 * time.Millisecond)

	server.Stop()
}

// 测试server.Wait是否正常
func TestServerPending(t *testing.T) {
	t.Run("test-server-pending-internal-interrupt", func(t *testing.T) {
		server := NewServer("test-server")
		testModule := &TestModule{}

		modules := []module.ModuleInfo{
			{
				Name:   "test-module",
				Module: testModule,
			},
		}

		server = server.RegModule(modules)

		server.Init(context.Background())
		server.Start(context.Background())

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		server.Wait(ctx)
		elapsed := time.Since(start)
		assert.Less(t, elapsed, 200*time.Millisecond)
	})
	t.Run("test-server-pending-terminate-interrupt", func(t *testing.T) {
		server := NewServer("test-server")
		testModule := &TestModule{}

		modules := []module.ModuleInfo{
			{
				Name:   "test-module",
				Module: testModule,
			},
		}

		server = server.RegModule(modules)

		server.Init(context.Background())
		server.Start(context.Background())

		start := time.Now()
		go func() {
			time.Sleep(100 * time.Millisecond)
			// 使用syscall发送中断信号
		}()
		server.Wait(context.Background())
		elapsed := time.Since(start)
		assert.Less(t, elapsed, 200*time.Millisecond)
	})
}

// 运行后进行grpc请求测试是否正常
func TestGrpcRequest(t *testing.T) {
	server := NewServer("test-server")
	testModule := &TestModule{}
	modules := []module.ModuleInfo{
		{
			Name:   "test-module",
			Module: testModule,
		},
	}
	port := 8080
	server = server.RegModule(modules)
	server.WithConfig(func() serverconfig.Config {
		return serverconfig.Config{Port: port}
	})
	server.Init(context.Background())
	server.Start(context.Background())

	conn, err := trpc.NewInternalClientWithHost("localhost:" + strconv.Itoa(port))
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	client := test.NewTestServiceClient(conn)
	resp, err := client.Test(context.Background(), &test.TestRequest{Name: "test"})
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "Hello, test", resp.Result)
}
