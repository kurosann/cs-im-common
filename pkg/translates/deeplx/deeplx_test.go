package deeplx

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTranslate(t *testing.T) {
	response := Translate(context.Background(), "Hello", "EN", "ZH")
	assert.Equal(t, int64(200), response.Code)
	t.Log(response)
}

func TestTranslateWithGo(t *testing.T) {
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := Translate(context.Background(), "Hello", "EN", "ZH")
			assert.Equal(t, int64(200), response.Code)
			t.Log(response)
		}()
	}
	wg.Wait()

	end := time.Now().Sub(start).Seconds()
	t.Log(fmt.Sprintf("time: %.2f", end))
}

func TestTranslateWithGoIsProxyUrl(t *testing.T) {
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := TranslateWithProxyUrl(context.Background(), "Hello", "EN", "ZH")
			assert.Equal(t, int64(200), response.Code)
			t.Log(response)
		}()
	}
	wg.Wait()

	end := time.Now().Sub(start).Seconds()
	t.Log(fmt.Sprintf("time: %.2f", end))
}

func TestTranslateWithGo300(t *testing.T) {
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < 300; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := Translate(context.Background(), "Hello", "EN", "ZH")
			assert.Equal(t, int64(200), response.Code)
			t.Log(response)
		}()
	}
	wg.Wait()

	end := time.Now().Sub(start).Seconds()
	t.Log(fmt.Sprintf("time: %.2f", end))
}

func TestTranslateByDeeplx(t *testing.T) {
	response := TranslateByDeeplx("{\"0\":\"签到\",\"1\":\"找男炮友\",\"2\":\"什么样的呢？\",\"3\":\"私信我\",\"4\":\"有没有万秀区的资源\",\"5\":\"签到\",\"6\":\"本条规则仅适用于【个人老师】 与【商家】两类成员，其他成员不受此限\\n\\n入群即代表遵守以下两条铁律：\\n\\n✅ 允许发：签到｜导航｜\\n\\n❌ 禁止发：推广｜任何形式广告\\n\\n⚠️ 违者直接禁言，不警告，不解封。\\n\\n每一次违规，都会增加大家错过重要信息的风险\\n感 谢理解与配合\",\"7\":\"‼️ 安排流程：\\n1. 联系指定电报客服\\n2. 看照片挑选老师\\n3. 客服安排客户去公寓\\n专注 楼凤 服务，“师资”丰富，众享“师”滑\\n熟女 少妇 少女 学生 你想要的服务我都有 !\\n需要服务请点击下方按钮查看👇\"}", "En", "zh", "")
	assert.Equal(t, int64(200), response.Code)
	t.Log(response)
}
