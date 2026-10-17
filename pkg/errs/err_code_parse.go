package errs

import (
	"regexp"
	"strconv"
)

// ParseErr 解析错误码
func ParseErr(errStr string) int {
	// 获取错误码
	// rpc error: code =Internal desc = err:10003 中 10003
	// 用正则匹配
	reg := regexp.MustCompile(`err: (\d+)`)
	match := reg.FindStringSubmatch(errStr)
	if len(match) > 1 {
		code, _ := strconv.Atoi(match[1])
		return code
	}
	reg = regexp.MustCompile(`err:(\d+)`)
	match = reg.FindStringSubmatch(errStr)
	if len(match) > 1 {
		code, _ := strconv.Atoi(match[1])
		return code
	}
	return -1
}
