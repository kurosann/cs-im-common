package consts

import (
	"errors"
	"strings"
)

type MethodType string

const (
	AddDomain         MethodType = "ADDDOMAIN"
	FindDomain        MethodType = "FINDDOMAIN"
	CheckDomain       MethodType = "CHECKDOMAIN"
	FindGroup         MethodType = "FINDGROUP"
	AddGroup          MethodType = "ADDGROUP"
	EditGroup         MethodType = "EDITGROUP"
	DelGroup          MethodType = "DELGROUP"
	AddSubAccount     MethodType = "ADDSUBACCOUNT"
	DelSubAccount     MethodType = "DELSUBACCOUNT"
	EditSubAccount    MethodType = "EDITSUBACCOUNT"
	FindSubAccount    MethodType = "FINDSUBACCOUNT"
	GetSubAccount     MethodType = "GETSUBACCOUNT"
	DistributionNums  MethodType = "DISTRIBUTIONNUMS"
	DistributionGroup MethodType = "DISTRIBUTIONGROUP"
	CheckSubAccName   MethodType = "CHECKSUBACCNAME"
	FreezeAccount     MethodType = "FREEZEACCOUNT"
	ForceLogout       MethodType = "FORCELOGOUT"
	GetAccount        MethodType = "GETACCOUNNT"
	EditAccountInfo   MethodType = "EDITACCOUNTINFO" // 编辑主账号
	SendMaileMsg      MethodType = "SENDMAILEMSG"    // 发送邮件
	CheckEmail        MethodType = "CHECKEMAIL"      // 检查邮件
	GetTime           MethodType = "GETTIME"         // 检查邮件
	Advertises        MethodType = "ADVERTISES"      // 获取广告
	PrvServerIp       MethodType = "PRVSERVERIP"     // 获取服务器IP
	Translate         MethodType = "TRANSLATE"       // 翻译
	AIGenerate        MethodType = "AIGENERATE"
	AIGenerateSnake   MethodType = "AI_GENERATE"
)

func (m MethodType) ToString() string {
	return string(m)
}

func StrToType(s string) MethodType {
	return MethodType(strings.ToUpper(s))
}

type MethodErr error

var (
	ErrMethodNotSupport MethodErr = errors.New("method not support")
)
