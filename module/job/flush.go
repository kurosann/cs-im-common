package job

import (
	"context"
	"sync"
)

var (
	jobMap = make(map[string]IJobTask)
	lock   sync.RWMutex
)

func RegisterFlush(name string, job IJobTask) {
	lock.Lock()
	defer lock.Unlock()
	jobMap[name] = job
}

func GetJob(name string) IJobTask {
	lock.RLock()
	defer lock.RUnlock()
	return jobMap[name]
}

func Flush(ctx context.Context, name string) {
	job := GetJob(name)
	if job != nil {
		job.Run(ctx)
	}
}
