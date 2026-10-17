package jsencry

import (
	"math/rand"
	"sync"
	"testing"
	"vane-common/pkg/dec"
)

func TestEncry(t *testing.T) {

	chanCount := make(chan struct{}, 200)

	swg := sync.WaitGroup{}
	for i := 0; i < 1000000; i++ {
		swg.Add(1)
		chanCount <- struct{}{}
		go func(j int) {
			defer func() {
				swg.Done()
				<-chanCount
			}()
			x := rand.Intn(10000)
			y := rand.Intn(10000)
			r := rand.Intn(100)
			code1, code2 := GenerateEncryCode()
			runner1 := New()
			runner1.Set("x", x)
			runner1.Set("y", y)
			runner1.Set("r", r)
			result, err := runner1.Run(code1)
			if err != nil {
				t.Error(err)
				return
			}

			res := result.([]interface{})
			runner2 := New()
			runner2.Set("z", dec.N(res[0]).IntPart())
			runner2.Set("R", dec.N(res[1]).IntPart())
			runner2.Set("G", dec.N(res[2]).IntPart())

			r2, err := runner2.Run(code2)
			if err != nil {
				t.Error(err)
				return
			}
			if r2 != nil && !r2.(bool) {
				t.Error(j)
				t.Error(r2)
				t.Log(res[2])
				return
			}
		}(i)
	}
	swg.Wait()
}
