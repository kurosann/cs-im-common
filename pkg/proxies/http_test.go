package proxies

import "testing"

// 创建测试用的 HTTP 代理配置对象
func createTestHttpConfig() HttpDialerConfig {
	return HttpDialerConfig{
		Host:                  "tokyo.link.ac.cn",
		Port:                  "995",
		Scheme:                "https", // 端口 995 通常用于 HTTPS 代理
		Username:              "1585457",
		Password:              "chU3fOyUrqk7",
		TLSInsecureSkipVerify: true,
	}
}
func TestHttpDialer(t *testing.T) {
	config := createTestHttpConfig()
	dialer, err := NewHttpDialer(config)
	if err != nil {
		t.Fatalf("创建 HttpDialer 失败: %v", err)
	}
	t.Run("Query IP", func(t *testing.T) {
		testSSIPQuery(t, dialer, "httpbin.org:80")
	})

	// 测试查询地区信息
	t.Run("Query Location", func(t *testing.T) {
		testSSLocationQuery(t, dialer, "ip-api.com:80")
	})
}
