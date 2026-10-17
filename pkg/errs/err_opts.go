package errs

import (
	"errors"
	"fmt"
)

// 错误码
type ServerErrCode int

// 通用错误码
const (
	ErrCodeOk               ServerErrCode = 10000 + iota // OK
	ErrCodeNotFoundCommAuth                              // 未找到通信认证 	// 占位符，跳过10002
	ErrCodeCheckReqError                                 // 检测到请求错误
)

// 账号相关错误码
const (
	ErrCodeNotFoundAcc                ServerErrCode = 12000 + iota // 账号不存在
	ErrCodeAccFrozen                                               // 账号已经被冻结
	ErrCodeProductNotFound                                         // 未找到产品
	ErrCodeProductExpired                                          // 产品已过期
	ErrCodeSubAccNotFound                                          // 未找到该子账号
	ErrCodeDomainNotFound                                          // 请先添加域名
	ErrCodeParseDataFailed                                         // 解析数据失败
	ErrCodeInvalidAcc                                              // 请选择正确的账号
	ErrCodeProductNotAdded                                         // 请先添加产品
	ErrCodeProductInsufficient                                     // 产品数量不足
	ErrCodeSignVerifyFailed                                        // 签名校验失败
	ErrCodeAccNameExists                                           // 账号名已存在
	ErrCodeAddSubAccFailed                                         // 添加子账号失败
	ErrCodeDistributionLengthMismatch                              // 子账号数量分配长度不一致
	ErrCodeDistributionNumsExceeded                                // 子账号数量分配数量大于可分配数量
	ErrCodeTokenIsError                                            // 凭证错误
	ErrCodeAccNameRequired                                         // 请输入账号
	ErrCodePasswordRequired                                        // 请输入密码
	ErrCodePortNumRequired                                         // 请输入分配数量
	ErrCodeAccIdRequired                                           // 请选择账号
	ErrCodeNoPermission                                            // 无权限
	ErrCodeQueryFailed                                             // 查询失败
	ErrCodeUserInfoNotFound                                        // 查询不到用户信息
	ErrCodeUserExpired                                             // 用户信息失效
	ErrCodeFreezeFailed                                            // 冻结失败
	ErrCodeForceLogoutFailed                                       // 强制退出失败
	ErrCodeUpdateFailed                                            // 更新失败
	ErrCodeDeleteFailed                                            // 删除失败
	ErrCodeAddFailed                                               // 添加失败
	ErrCodeNotFound                                                // 找不到
	ErrCodeParamError                                              // 参数错误
	ErrCodeDomainExists                                            // 该账号已添加域名
	ErrCodeDomainRequired                                          // 请输入域名
	ErrCodeSuffixRequired                                          // 请输入后缀
	ErrCodeAccLoginError                                           // 账号登录异常
	ErrCodeEmailRequired                                           // 请输入邮箱
	ErrCodeEmailExists                                             // 该邮箱已存在
	ErrCodeEmailSendTooFrequently                                  // 验证码发送过于频繁
	ErrAccLoginAccOrPwdError                                       // 账号或密码错误
	ErrCodeNoProductPermission                                     // 未有产品权限
	ErrCodeGetTokenError                                           // 服务器获取凭证异常
	ErrServerSignFailure                                           // 签名失败
	ErrServerBaseError                                             // 服务器繁忙
	ErrLoginCountError                                             // 登录错误次数过多,请过几分钟后再登录

)

func NewErr(code ServerErrCode) error {
	return errors.New(fmt.Sprintf("%d", code))
}

// GetErrMsgByCode 根据错误码获取错误信息
func GetErrMsgByCode(code int) string {
	if code == -1 {
		return ""
	}
	errCode := ServerErrCode(code)

	switch errCode {
	case ErrCodeOk:
		return "OK"
	case ErrCodeNotFoundCommAuth:
		return "未找到通信认证"
	case ErrCodeNotFoundAcc:
		return "账号不存在"
	case ErrCodeCheckReqError:
		return "检测到请求错误"
	case ErrCodeAccFrozen:
		return "账号已经被冻结"
	case ErrCodeProductNotFound:
		return "未找到产品"
	case ErrCodeProductExpired:
		return "产品已过期"
	case ErrCodeParseDataFailed:
		return "解析数据失败"
	case ErrCodeSubAccNotFound:
		return "未找到该子账号"
	case ErrCodeInvalidAcc:
		return "请选择正确的账号"
	case ErrCodeDomainNotFound:
		return "请先添加域名"
	case ErrCodeProductNotAdded:
		return "请先添加产品"
	case ErrCodeProductInsufficient:
		return "超过最大端口数量"
	case ErrCodeAccNameExists:
		return "账号名已存在"
	case ErrCodeAddSubAccFailed:
		return "添加子账号失败"
	case ErrCodeDistributionLengthMismatch:
		return "子账号数量分配长度不一致"
	case ErrCodeDistributionNumsExceeded:
		return "子账号数量分配数量大于可分配数量"
	case ErrCodeSignVerifyFailed:
		return "签名校验失败"
	case ErrCodeAccNameRequired:
		return "请输入账号"
	case ErrCodePasswordRequired:
		return "请输入密码"
	case ErrCodePortNumRequired:
		return "请输入分配数量"
	case ErrCodeAccIdRequired:
		return "请选择账号"
	case ErrCodeNoPermission:
		return "无权限"
	case ErrCodeQueryFailed:
		return "查询失败"
	case ErrCodeUserInfoNotFound:
		return "查询不到用户信息"
	case ErrCodeUserExpired:
		return "用户信息获取失效"
	case ErrCodeFreezeFailed:
		return "冻结失败"
	case ErrCodeForceLogoutFailed:
		return "强制退出失败"
	case ErrCodeUpdateFailed:
		return "更新失败"
	case ErrCodeDeleteFailed:
		return "删除失败"
	case ErrCodeAddFailed:
		return "添加失败"
	case ErrCodeNotFound:
		return "找不到"
	case ErrCodeParamError:
		return "参数错误"
	case ErrCodeDomainExists:
		return "该账号已添加域名"
	case ErrCodeDomainRequired:
		return "请输入域名"
	case ErrCodeSuffixRequired:
		return "请输入后缀"
	case ErrCodeTokenIsError:
		return "凭证错误"
	case ErrCodeAccLoginError:
		return "账号登录异常"
	case ErrCodeGetTokenError:
		return "服务器验证凭证异常"
	case ErrCodeEmailRequired:
		return "验证码错误"
	case ErrCodeEmailExists:
		return "邮箱已经存在"
	case ErrCodeEmailSendTooFrequently:
		return "验证码发送过于频繁，请稍后再试"
	case ErrAccLoginAccOrPwdError:
		return "账号或密码错误"
	case ErrCodeNoProductPermission:
		return "未有产品权限"
	case ErrServerBaseError:
		return "服务器繁忙"
	case ErrServerSignFailure:
		return "签名失败"
	case ErrLoginCountError:
		return "登录错误次数过多,请过一段时间后再重新登录"
	default:
		return "未知错误"
	}
}
