package deeplx

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

func fetchUri() string {
	if len(urls) < 1 {

		var wgs sync.WaitGroup
		wgs.Add(2)
		for i, url := range []string{
			"https://github.com/xiaozhou26/serch_deeplx/blob/main/success.txt",
		} {
			go fetchUrls(&wgs, i, url)
		}
		wgs.Wait()

		for _, url := range targetUrls {
			wg.Add(1)
			go checkUrlVerify(url, &wg)
		}
		wg.Wait()
	}
	if len(urls) < 1 {
		return ""
	}
	randomIndex := randomNum(urls)
	if randomIndex <= len(urls) {
		return urls[randomIndex]
	}

	return urls[0]
}

var client = &http.Client{
	Timeout: 3 * time.Second,
}

func fetchUrls(wg *sync.WaitGroup, k int, url string) {
	defer wg.Done()

	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	r := bufio.NewReader(resp.Body)
	for {
		line, _, errs := r.ReadLine()
		if errs == io.EOF {
			break
		}

		newUrl := string(line)
		if k == 1 {
			newUrl = fmt.Sprintf("%s/translate", newUrl)
		}
		targetUrls = append(targetUrls, newUrl)
	}
}

func checkUrlVerify(url string, wg *sync.WaitGroup) {
	defer wg.Done()

	resp, err := client.Get(strings.ReplaceAll(url, "/translate", ""))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		urls = append(urls, url)
	}
}

func randomNum(slices []string) int {
	urlsLen := len(slices)
	return rand.Intn(urlsLen)
}

var (
	proxyUrls []string
	once      sync.Once
)

func getProxyUrl() string {
	once.Do(func() {
		res, err := client.Get("https://269900.xyz/fetch_http_all")
		if err != nil {
			return
		}

		defer res.Body.Close()
		item, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return
		}

		f := bufio.NewReader(strings.NewReader(string(item)))
		for {
			line, err := f.ReadString('\n')
			if err != nil {
				break
			}

			proxyUrls = append(proxyUrls, strings.Trim(line, ","))
		}
	})

	if len(proxyUrls) < 1 {
		return ""
	}

	return proxyUrls[randomNum(proxyUrls)]
}

// getICount returns the number of 'i' characters in the text
func getICount(translateText string) int64 {
	return int64(strings.Count(translateText, "i"))
}

// getRandomNumber generates a random number for request ID
func getRandomNumber() int64 {
	src := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(src)
	num := rng.Int63n(99999) + 100000
	return num * 1000
}

// getTimeStamp generates timestamp for request based on i count
func getTimeStamp(iCount int64) int64 {
	ts := time.Now().UnixMilli()
	if iCount != 0 {
		iCount = iCount + 1
		return ts - (ts % iCount) + iCount
	}
	return ts
}

// formatPostString formats the request JSON string with specific spacing rules
func formatPostString(postData *PostData) string {
	postBytes, _ := json.Marshal(postData)
	postStr := string(postBytes)
	return postStr
}

// handlerBodyMethod manipulates the request body based on random number calculation
func handlerBodyMethod(random int64, body string) string {
	calc := (random+5)%29 == 0 || (random+3)%13 == 0
	if calc {
		return strings.Replace(body, `"method":"`, `"method" : "`, 1)
	}
	return strings.Replace(body, `"method":"`, `"method": "`, 1)
}
