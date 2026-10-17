package module

import (
	"context"
	"time"

	"vane-common/module/job"
	"vane-common/module/monitor"
	"vane-common/module/statemgr"
	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/taskmgr"

	"google.golang.org/grpc"
)

type IModuleDb interface {
	Model() []ModelRegInfo
}
type IPublicModule interface {
	Public() bool
}
type IModuleStateMgr interface {
	StateMgr() []statemgr.StateSyncRegInfo
}
type IMonitorModule interface {
	Monitor() []monitor.MonitorRegInfo
}
type IStopModule interface {
	Stop(ctx context.Context) error
}

type IModule interface {
	Init(ctx context.Context) error
	Api() []ApiRegInfo
	Job() []JobRegInfo
	Task() []TaskRegInfo
}

type ModelRegInfo struct {
	Model    model.Table                    // 接口具体实现
	CacheTtl time.Duration                  // 缓存过期时间 0表示不缓存
	Listen   ModelListenOption[model.Table] // 数据变化监听函数
}

type ModelOption[T model.Table] interface {
	apply(model *ModelRegInfo)
}

type modelCacheOption struct {
	cacheTtl time.Duration
}

// WithModelCache 设置缓存过期时间 0表示不缓存
func WithModelCache(cacheTtl time.Duration) ModelOption[model.Table] {
	return &modelCacheOption{cacheTtl: cacheTtl}
}

func (o *modelCacheOption) apply(model *ModelRegInfo) {
	model.CacheTtl = o.cacheTtl
}

type modelListenOption[T model.Table] struct {
	listen ModelListenOption[T] // 数据变化监听函数
}
type ModelListenOption[T model.Table] interface {
	Listen(item dataaccess.ListenItem[T])
}

type ModelListenFunc[T model.Table] func(item dataaccess.ListenItem[T])

func (f ModelListenFunc[T]) Listen(item dataaccess.ListenItem[T]) {
	f(item)
}

// WithModelListen 设置数据变化监听函数
func WithModelListen[T model.Table](listen ModelListenOption[T]) ModelOption[T] {
	return &modelListenOption[T]{listen: listen}
}

func (o *modelListenOption[T]) apply(info *ModelRegInfo) {
	info.Listen = ModelListenFunc[model.Table](func(item dataaccess.ListenItem[model.Table]) {
		o.listen.Listen(dataaccess.ListenItem[T]{Model: item.Model.(T), Action: item.Action})
	})
}

// NewModelRegInfo 创建一个Model注册信息
// model: 接口具体实现
func NewModelRegInfo[T model.Table](model T, options ...ModelOption[T]) ModelRegInfo {
	modelInfo := ModelRegInfo{Model: model}
	for _, option := range options {
		option.apply(&modelInfo)
	}
	return modelInfo
}

// Api注册信息
type ApiRegInfo struct {
	Desc  grpc.ServiceDesc // grpc描述
	Proto any              // 接口具体实现
}

// NewApiRegInfo 创建一个Api注册信息
// desc: grpc描述
// proto: 接口具体实现
func NewApiRegInfo(desc grpc.ServiceDesc, proto any) ApiRegInfo {
	return ApiRegInfo{
		Desc:  desc,
		Proto: proto,
	}
}

// Job注册信息

type JobOption interface {
	apply(job *JobRegInfo)
}

var _ JobOption = (*jobLaunchRunOption)(nil)

type jobLaunchRunOption struct {
	launchRun bool
}

func WithJobLaunchRun() JobOption {
	return &jobLaunchRunOption{launchRun: true}
}

func (o *jobLaunchRunOption) apply(job *JobRegInfo) {
	job.LaunchRun = o.launchRun
}

type jobGlobalOption struct {
	isGlobal bool
}

func WithJobGlobal() JobOption {
	return &jobGlobalOption{isGlobal: true}
}

func (o *jobGlobalOption) apply(job *JobRegInfo) {
	job.IsGlobal = o.isGlobal
}

type JobRegInfo struct {
	Name      string       // 定时任务名称
	Spec      string       // cron表达式或者运行逻辑匹配
	Job       job.IJobTask // job 实现
	IsGlobal  bool         // 是否为全局单例任务
	LaunchRun bool         // 是否为启动时运行
}

// NewJobRegInfo 创建一个Job注册信息
// name: 定时任务名称
// spec: cron表达式或者运行逻辑匹配
// job: job 实现
// isGlobal: 是否为全局单例任务
// launchRun: 是否为启动时运行
func NewJobRegInfo(name string, spec string, job job.IJobTask, options ...JobOption) JobRegInfo {
	jobInfo := JobRegInfo{
		Name: name,
		Spec: spec,
		Job:  job,
	}
	for _, option := range options {
		option.apply(&jobInfo)
	}
	return jobInfo
}

// Task 注册信息
type TaskRegInfo struct {
	Code string             // 分布式事务计划代号
	Task taskmgr.TaskWorker // 任务编排
}

// NewTaskRegInfo 创建一个Task注册信息
// code: 分布式事务计划代号
// wallet: 任务编排
func NewTaskRegInfo(code string, task taskmgr.TaskWorker) TaskRegInfo {
	return TaskRegInfo{
		Code: code,
		Task: task,
	}
}

type ModuleInfo struct {
	Name   string
	Module IModule
}

// NewModule 创建一个模块信息
// name: 模块名称
// module: 模块实现
func NewModule(name string, module IModule) ModuleInfo {
	return ModuleInfo{
		Name:   name,
		Module: module,
	}
}
