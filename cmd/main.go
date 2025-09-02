package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/urfave/cli/v2"
)

const (
	email    = "john@example.com"
	password = "pwd"
	account  = "onward"
)

var requestsPerEndpoint int

func main() {
	app := &cli.App{
		Name:   "Spam Landscape",
		Usage:  "Spam Landscape Server with API calls to restart and remove computers",
		Action: actionSpam,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "id",
				Usage: "The id of the computer to spam. Ignored if ids is provided",
			},
			&cli.IntSliceFlag{
				Name:  "ids",
				Usage: "The ids of the computers to spam",
			},
			&cli.IntFlag{
				Name:        "requests",
				Aliases:     []string{"r"},
				Usage:       "The amount of requests to make to both the request and remove computer(s) endpoints.",
				Destination: &requestsPerEndpoint,
				Value:       10,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func actionSpam(ctx *cli.Context) error {
	jwt, err := login()
	if err != nil {
		return fmt.Errorf("error logging in: %v", err)
	}

	computerIds := ctx.IntSlice("ids")
	if len(computerIds) > 0 {
		return spamMany(ctx, jwt, computerIds)
	} else if computerId := ctx.Int("id"); computerId > 0 {
		return spamOne(ctx, jwt, computerId)
	}

	return fmt.Errorf("no computer ID(s) provided")
}

type LoginResponse struct {
	Token string `json:"token"`
}

func login() (string, error) {
	loginData := map[string]string{"email": email, "password": password, "account": account}
	jsonBody, err := json.Marshal(loginData)
	if err != nil {
		log.Fatalf("error marshalling login data: %v", err)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	res, err := client.Post("http://localhost:9091/api/login", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var loginResponse LoginResponse

	if err := json.Unmarshal(body, &loginResponse); err != nil {
		return "", err
	}

	jwt := loginResponse.Token

	if jwt == "" {
		return jwt, fmt.Errorf("error: JWT not found")
	}

	return jwt, nil
}

func spamOne(ctx *cli.Context, jwt string, computerId int) error {
	var wg sync.WaitGroup

	for i := 0; i < requestsPerEndpoint; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			restartOneComputer(jwt, computerId)
		}()
		go func() {
			defer wg.Done()
			removeOneComputer(jwt, computerId)
		}()
	}

	wg.Wait()
	return nil
}

func spamMany(ctx *cli.Context, jwt string, computerIds []int) error {
	var wg sync.WaitGroup

	for i := 0; i < requestsPerEndpoint; i++ {
		wg.Add(2)
		go func(ids []int) {
			defer wg.Done()
			restartManyComputers(jwt, ids)
		}(computerIds)
		go func(ids []int) {
			defer wg.Done()
			removeManyComputers(jwt, ids)
		}(computerIds)
	}

	wg.Wait()
	return nil
}

func restartOneComputer(jwt string, computerId int) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := fmt.Sprintf("http://localhost:9091/api/computers/%d/restart", computerId)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Printf("error creating request: %v", err)
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))

	res, err := client.Do(req)
	if err != nil {
		log.Printf("error making request: %v", err)
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("error reading body: %v", err)
		return err
	}

	log.Printf("restart one computer response: %s", fmt.Sprintf("%s, %d", string(body), res.StatusCode))

	return nil
}

func restartManyComputers(jwt string, computerIds []int) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "http://localhost:9091/api/computers/restart"
	jsonBody := map[string][]int{"computer_ids": computerIds}
	marshalledBody, err := json.Marshal(jsonBody)
	if err != nil {
		log.Printf("error marshalling request body: %v", err)
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(marshalledBody))
	if err != nil {
		log.Printf("error creating request: %v", err)
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		log.Printf("error making request: %v", err)
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("error reading body: %v", err)
		return err
	}

	log.Printf("restart many computers response: %s", fmt.Sprintf("%s, %d", string(body), res.StatusCode))
	return nil
}

func removeOneComputer(jwt string, computerId int) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("GET",
		fmt.Sprintf("http://localhost:9091/api?action=RemoveComputers&version=2011-08-01&computer_ids.1=%d", computerId),
		nil)
	if err != nil {
		log.Printf("error creating request: %v", err)
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))

	res, err := client.Do(req)
	if err != nil {
		log.Printf("error making request: %v", err)
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("error reading response body: %v", err)
		return err
	}

	log.Printf("remove one computer response: %s", fmt.Sprintf("%s, %d", string(body), res.StatusCode))
	return nil
}

func removeManyComputers(jwt string, computerIds []int) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	var idQueryString string

	for i, id := range computerIds {
		if i == 0 {
			idQueryString += fmt.Sprintf("computer_ids.%d=%d", i+1, id)
		} else {
			idQueryString += fmt.Sprintf("&computer_ids.%d=%d", i+1, id)
		}
	}

	req, err := http.NewRequest("GET",
		fmt.Sprintf("http://localhost:9091/api?action=RemoveComputers&version=2011-08-01&%s", idQueryString),
		nil)
	if err != nil {
		log.Printf("error creating request: %v", err)
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))

	res, err := client.Do(req)
	if err != nil {
		log.Printf("error making request: %v", err)
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("error reading response body: %v", err)
		return err
	}

	log.Printf("remove many computers response: %s", fmt.Sprintf("%s, %d", string(body), res.StatusCode))
	return nil
}
