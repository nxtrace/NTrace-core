package pow

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"
)

//TODO: 在这里要实现优选IP

type Challenge struct {
	RequestID string `json:"request_id"`
	Challenge string `json:"challenge"`
}

type RequestResponse struct {
	Challenge   Challenge `json:"challenge"`
	RequestTime int64     `json:"request_time"`
}

type SubmitRequest struct {
	Challenge   Challenge `json:"challenge"`
	Answer      []string  `json:"answer"`
	RequestTime int64     `json:"request_time"`
}

type SubmitResponse struct {
	Token string `json:"token"`
}

type GetTokenParams struct {
	TimeoutSec  time.Duration
	BaseUrl     string
	RequestPath string
	SubmitPath  string
	UserAgent   string
	SNI         string
	Host        string
}

func NewGetTokenParams() *GetTokenParams {
	return &GetTokenParams{
		TimeoutSec:  5 * time.Second, // 你的默认值
		BaseUrl:     "http://127.0.0.1:55000",
		RequestPath: "/request_challenge",
		SubmitPath:  "/submit_answer",
		UserAgent:   "POW client",
		SNI:         "",
		Host:        "",
	}
}

func RetToken(getTokenParams *GetTokenParams) (string, error) {
	// Get challenge
	challengeResponse, err := requestChallenge(getTokenParams)
	if err != nil {
		return "", err
	}
	//fmt.Println(challengeResponse.Challenge.Challenge)

	// Solve challenge and submit answer
	token, err := submitAnswer(getTokenParams, challengeResponse)
	if err != nil {
		return "", err
	}

	return token, nil
}

func requestChallenge(getTokenParams *GetTokenParams) (*RequestResponse, error) {
	client := &http.Client{
		Timeout: getTokenParams.TimeoutSec,
	}
	if getTokenParams.SNI != "" {
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					ServerName: getTokenParams.SNI,
				},
			},
		}
	}
	req, err := http.NewRequest("GET", getTokenParams.BaseUrl+getTokenParams.RequestPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", getTokenParams.UserAgent)
	//req.Header.Add("Host", getTokenParams.Host)
	req.Host = getTokenParams.Host
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)
	var challengeResponse RequestResponse
	err = json.NewDecoder(resp.Body).Decode(&challengeResponse)
	if err != nil {
		return nil, err
	}

	return &challengeResponse, nil
}

func submitAnswer(getTokenParams *GetTokenParams, challengeResponse *RequestResponse) (string, error) {
	client := &http.Client{
		Timeout: getTokenParams.TimeoutSec,
	}
	if getTokenParams.SNI != "" {
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					ServerName: getTokenParams.SNI,
				},
			},
		}
	}
	requestTime := challengeResponse.RequestTime
	challenge := challengeResponse.Challenge.Challenge
	requestId := challengeResponse.Challenge.RequestID
	N := new(big.Int)
	N.SetString(challenge, 10)
	factorsList := factors(N)
	if len(factorsList) != 2 {
		return "", errors.New("factors function did not return exactly two factors")
	}
	p1 := factorsList[0]
	p2 := factorsList[1]
	if p1.Cmp(p2) > 0 { // if p1 > p2
		p1, p2 = p2, p1 // swap p1 and p2
	}
	submitRequest := SubmitRequest{
		Challenge:   Challenge{RequestID: requestId},
		Answer:      []string{p1.String(), p2.String()},
		RequestTime: requestTime,
	}
	requestBody, err := json.Marshal(submitRequest)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", getTokenParams.BaseUrl+getTokenParams.SubmitPath, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("User-Agent", getTokenParams.UserAgent)
	//req.Header.Add("Host", getTokenParams.Host)
	req.Host = getTokenParams.Host

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return "", errors.New(string(bodyBytes))
	}

	var submitResponse SubmitResponse
	err = json.NewDecoder(resp.Body).Decode(&submitResponse)
	if err != nil {
		return "", err
	}

	return submitResponse.Token, nil
}
