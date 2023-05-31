package pow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xgadget-lab/nexttrace/config"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"runtime"
	"time"
)

const (
	baseURL     = "https://api.leo.moe/v3/challenge" // replace with your server url
	requestPath = "/request_challenge"
	submitPath  = "/submit_answer"
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

func GetToken() (string, error) {
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	// Get challenge
	challengeResponse, err := requestChallenge(client)
	if err != nil {
		return "", err
	}
	fmt.Println(challengeResponse.Challenge.Challenge)

	// Solve challenge and submit answer
	token, err := submitAnswer(client, challengeResponse)
	if err != nil {
		return "", err
	}

	return token, nil
}

func requestChallenge(client *http.Client) (*RequestResponse, error) {
	req, err := http.NewRequest("GET", baseURL+requestPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", fmt.Sprintf("NextTrace %s/%s/%s", config.Version, runtime.GOOS, runtime.GOARCH))

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

func submitAnswer(client *http.Client, challengeResponse *RequestResponse) (string, error) {
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

	req, err := http.NewRequest("POST", baseURL+submitPath, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("User-Agent", fmt.Sprintf("NextTrace %s/%s/%s", config.Version, runtime.GOOS, runtime.GOARCH))

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
