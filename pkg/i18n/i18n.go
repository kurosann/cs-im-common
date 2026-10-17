package i18n

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

var once sync.Once

type Language string

type TransCode string

const (
	ZhCN Language = "zh-cn"
	ZhTW Language = "zh-tw"
	EnUS Language = "en-us"
	RuRU Language = "ru-ru"
)
const DefaultLanguage = EnUS

var (
	//字典
	transMap = map[TransCode]map[Language]string{}
)

func GetTransMap() map[TransCode]map[Language]string {
	return transMap
}

// GetTransText 获取翻译文本
func GetTransText(key string, lang Language) string {
	if arr, ok := transMap[TransCode(key)]; ok {
		return getLangVal(arr, lang)
	}
	return ""
}

// GetTransTextDefault 获取翻译文本， 如果没有翻译则返回默认文本
func GetTransTextDefault(key string, lang Language, defaultText string) string {
	text := GetTransText(key, lang)
	if text == "" {
		return defaultText
	}
	return text
}

// 获取对应语言的值 不存在取第一个元素
func getLangVal(arr map[Language]string, lang Language) (result string) {
	defer func() {
		if err := recover(); len(arr) != 0 && (err != nil || result == "") {
			result = arr[DefaultLanguage]
		}
	}()
	return arr[lang]
}

// GenParamsHolder 生成参数占位 key -> ${key}
func GenParamsHolder(key string) string {
	return fmt.Sprintf("${%s}", key)
}

// SetParam 将文本内的参数占位替换为所需值 ${key} -> val
func SetParam(text, key string, lang Language) string {
	return strings.ReplaceAll(text, GenParamsHolder(key), GetTransText(key, lang))
}

// 翻译所有占位文本
func TransText(text string, lang Language) string {
	for key := range transMap {
		text = SetParam(text, string(key), lang)
	}
	return text
}

func InitI18n(transDict map[TransCode]map[Language]string) {
	once.Do(func() { transMap = transDict })
}

type ctxKey string

const (
	LangCtxKey ctxKey = "x-lang"
)

func GetLang(ctx context.Context) Language {
	lang := ctx.Value(LangCtxKey)
	if lang == nil {
		return DefaultLanguage
	}
	return lang.(Language)
}

func WithLangCtx(ctx context.Context, lang Language) context.Context {
	return context.WithValue(ctx, LangCtxKey, lang)
}
