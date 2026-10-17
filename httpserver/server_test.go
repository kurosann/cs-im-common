package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReqContextBindQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("GET", "/test?name=kuro&age=18&enabled=true&tags=a&tags=b", nil)
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = req
	ctx := &reqContext{ctx: ginCtx}

	type queryDTO struct {
		Name    string   `query:"name"`
		Age     int      `query:"age"`
		Enabled bool     `query:"enabled"`
		Tags    []string `query:"tags"`
	}

	var dto queryDTO
	err := ctx.BindQuery(&dto)
	require.NoError(t, err)
	require.Equal(t, "kuro", dto.Name)
	require.Equal(t, 18, dto.Age)
	require.True(t, dto.Enabled)
	require.Equal(t, []string{"a", "b"}, dto.Tags)
}

func TestReqContextBindForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("POST", "/test", strings.NewReader("name=kuro&age=18"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = req
	ctx := &reqContext{ctx: ginCtx}

	type formDTO struct {
		Name string `form:"name"`
		Age  int    `form:"age"`
	}

	var dto formDTO
	err := ctx.BindForm(&dto)
	require.NoError(t, err)
	require.Equal(t, "kuro", dto.Name)
	require.Equal(t, 18, dto.Age)
}

func TestReqContextSetValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = req
	ctx := &reqContext{ctx: ginCtx}

	ctx.SetValue("user_id", "u-1")

	require.Equal(t, "u-1", ctx.Value("user_id"))
	require.Equal(t, "u-1", ctx.Context().Value("user_id"))
}

func TestReqContextParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/session/123", nil)
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = req
	ginCtx.Params = gin.Params{{Key: "id", Value: "123"}}
	ctx := &reqContext{ctx: ginCtx}

	require.Equal(t, "123", ctx.Param("id"))
}

func TestReqContextFailData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = req
	ctx := &reqContext{ctx: ginCtx}

	ctx.FailData(http.StatusUnauthorized, "no auth", map[string]any{"reason": "token"})

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"ok":false`)
	require.Contains(t, recorder.Body.String(), `"message":"no auth"`)
	require.Contains(t, recorder.Body.String(), `"reason":"token"`)
}

func TestReqContextCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "abc"})
	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	ginCtx.Request = req
	ctx := &reqContext{ctx: ginCtx}

	value, err := ctx.Cookie("token")
	require.NoError(t, err)
	require.Equal(t, "abc", value)

	ctx.SetCookie(&http.Cookie{Name: "sid", Value: "s1", Path: "/"})
	require.Contains(t, recorder.Header().Get("Set-Cookie"), "sid=s1")
}
