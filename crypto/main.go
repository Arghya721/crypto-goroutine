package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var wg sync.WaitGroup

type coinDetails struct {
	Coin                  string            `json:"coin"`
	Price                 string            `json:"price"`
	Rank                  int               `json:"rank"`
	Symbol                string            `json:"symbol"`
	Image                 string            `json:"image"`
	Link                  string            `json:"link"`
	MarketCap             string            `json:"market_cap"`
	FullyDilutedMarketCap string            `json:"fully_diluted_market_cap"`
	Volume24h             string            `json:"volume_24h"`
	VolumeByMarketCap     string            `json:"volume_by_market_cap"`
	CirculatingSupply     string            `json:"circulating_supply"`
	HighLow24hr           map[string]string `json:"high_low_24hr"`
	MaxSupply             string            `json:"max_supply"`
	TotalSupply           string            `json:"total_supply"`
}

func getCoinDetails(url string) coinDetails {
	//fmt.Println(url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(err)
	}
	res, _ := http.DefaultClient.Do(req)
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	var coinDetailsRegex = regexp.MustCompile(`(?m)<div\s*class="sc-1prm8qw-0\s*cVuwoC\s*nameHeader"><img\s*src="(.*?)".*?<span\s*class="sc-169cagi-0\s*kQxZxB.*?data-text="(.*?)".*?class="nameSymbol">(.*?)<\/small>.*?namePill\s*namePillPrimary">Rank\s*#([0-9]*).*?priceValue.*?<span>(.*?)</span>.*?n78udj-5\s*dBJPYV"><span>(.*?)</span>.*?n78udj-5\s*dBJPYV"><span>(.*?)<\/span>.*?statsValue">(.*?)</div>.*?statsValue">(.*?)</div>.*?statsValue">(.*?)</div>.*?statsValue">(.*?)</div>.*?link-button.*?href="(.*?)"`)

	match := coinDetailsRegex.FindAllStringSubmatch(string(body), -1)

	var maxSupplyRegex = regexp.MustCompile(`(?m)class="maxSupplyValue">(.*?)</div>.*?maxSupplyValue">(.*?)</div>`)

	maxSupplyMatch := maxSupplyRegex.FindAllStringSubmatch(string(body), -1)

	var maxSupply string
	var totalSupply string

	if maxSupplyMatch != nil {
		maxSupply = maxSupplyMatch[0][1]
		totalSupply = maxSupplyMatch[0][2]
	} else {
		maxSupply = "N/A"
		totalSupply = "N/A"
	}

	rank, err := strconv.Atoi(match[0][4])
	if err != nil {
		fmt.Println(err)
	}

	return coinDetails{
		Coin:                  match[0][2],
		Price:                 match[0][5],
		Rank:                  rank,
		Symbol:                match[0][3],
		Image:                 match[0][1],
		Link:                  match[0][13],
		MarketCap:             match[0][8],
		FullyDilutedMarketCap: match[0][9],
		Volume24h:             match[0][10],
		VolumeByMarketCap:     match[0][11],
		CirculatingSupply:     match[0][12],
		HighLow24hr: map[string]string{
			"high": match[0][7],
			"low":  match[0][6],
		},
		MaxSupply:   maxSupply,
		TotalSupply: totalSupply,
	}
}

func extractURL(html string) (urls []string) {
	var coinNameRegex = regexp.MustCompile(`(?m)<a\s*href="\/currencies\/([a-zA-Z-]*)\/"\s*class="cmc-link">`)

	for _, match := range coinNameRegex.FindAllStringSubmatch(string(html), -1) {
		urls = append(urls, "https://coinmarketcap.com/currencies/"+match[1])
	}
	return urls
}

func urlList() []coinDetails {
	req, err := http.NewRequest("GET", "https://coinmarketcap.com/coins/", nil)
	if err != nil {
		fmt.Println(err)
	}
	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	urlList := extractURL(string(body))

	var coinDetailsList []coinDetails

	// implement go routines to get coin details for each url
	for _, url := range urlList {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			coinDetailsList = append(coinDetailsList, getCoinDetails(url))

		}(url)
	}
	wg.Wait()

	return coinDetailsList
}

type Response events.APIGatewayProxyResponse

func Handler(ctx context.Context) (Response, error) {
	var buf bytes.Buffer

	allCoinDetails := urlList()

	// sort coinDetailsList by rank
	sort.Slice(allCoinDetails[:], func(i, j int) bool {
		return allCoinDetails[i].Rank < allCoinDetails[j].Rank
	})

	body, err := json.Marshal(map[string]interface{}{
		"Data": allCoinDetails,
	})
	if err != nil {
		return Response{StatusCode: 404}, err
	}
	json.HTMLEscape(&buf, body)

	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            buf.String(),
		Headers: map[string]string{
			"Content-Type":           "application/json",
			"X-MyCompany-Func-Reply": "hello-handler",
		},
	}
	return resp, nil
}

func main() {
	lambda.Start(Handler)
}
