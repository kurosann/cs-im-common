package jsencry

import (
	"github.com/dop251/goja"
)

// JSRunner 封装 goja VM
type JSRunner struct {
	vm *goja.Runtime
}

// New 创建一个新的 JS 执行器
func New() *JSRunner {
	return &JSRunner{
		vm: goja.New(),
	}
}

// Run 执行 JS 代码并返回结果
func (r *JSRunner) Run(code string) (interface{}, error) {
	v, err := r.vm.RunString(code)
	if err != nil {
		return nil, err
	}
	return v.Export(), nil
}

// Set 注入 Go 对象/函数到 JS 环境
func (r *JSRunner) Set(name string, value interface{}) error {
	return r.vm.Set(name, value)
}

// Get 获取 JS 全局变量
func (r *JSRunner) Get(name string) (interface{}, error) {
	val := r.vm.Get(name)
	if val == nil {
		return nil, nil
	}
	return val.Export(), nil
}
