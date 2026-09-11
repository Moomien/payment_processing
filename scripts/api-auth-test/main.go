package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	email := randomEmail()
	username := randomUsername()
	password := "testpass"

	jsonData := `{
		"email": "` + email + `",
		"username": "` + username + `",
		"password": "` + password + `"
	}`

	//register
	resp, err := http.Post("http://localhost:8080/auth/register", "application/json", bytes.NewBuffer([]byte(jsonData)))
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Println("Register response:", string(body))

	if resp.StatusCode != 201 {
		log.Println(resp.Body)
		return
	}

	access_token := accessToken(resp)
	refresh_token := refreshToken(resp)
	log.Println("Register success: ", access_token)

	//login
	resp, err = http.Post("http://localhost:8080/auth/login", "application/json", bytes.NewBuffer([]byte(jsonData)))
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Login success!")

	//refresh endpoint
	refreshData := `{"refresh_token": "` + refresh_token + `"}`
	resp, err = http.Post("http://localhost:8080/auth/refresh", "application/json", bytes.NewBuffer([]byte(refreshData)))
	if err != nil {
		log.Println(err)
		return
	}

	//logout
	req, err := http.NewRequest("POST", "http://localhost:8080/auth/logout", nil)
	if err != nil {
		log.Println(err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+refresh_token)
	req.Header.Set("Cookie", "refresh_token="+refresh_token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	if resp.StatusCode != 200 {
		log.Println("Logout failed with status:", resp.StatusCode)
		return
	}
	log.Println("Logout success!")

	//register
	email = randomEmail()
	username = randomUsername()
	password = "testpass"

	jsonData = `{
		"email": "` + email + `",
		"username": "` + username + `",
		"password": "` + password + `"
	}`

	resp, err = http.Post("http://localhost:8080/auth/register", "application/json", bytes.NewBuffer([]byte(jsonData)))
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	log.Println("Register response:", string(body))

	if resp.StatusCode != 201 {
		log.Println(resp.Body)
		return
	}

	refresh_token = refreshToken(resp)

	//logout-all
	req, err = http.NewRequest("POST", "http://localhost:8080/auth/logout-all", nil)
	if err != nil {
		log.Println(err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+refresh_token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	if resp.StatusCode != 200 {
		log.Println("Logout-all failed with status:", resp.StatusCode)
		return
	}
	log.Println("Logout-all success!")
}

func randomEmail() string {
	return fmt.Sprintf("user_%d@example.com", time.Now().Unix())
}

func randomUsername() string {
	return fmt.Sprintf("user_%d", time.Now().Unix())
}

func accessToken(r *http.Response) string {
	var data struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}

	_ = json.NewDecoder(r.Body).Decode(&data)
	return data.Tokens.AccessToken
}

func refreshToken(r *http.Response) string {
	var data struct {
		Tokens struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	}

	_ = json.NewDecoder(r.Body).Decode(&data)
	return data.Tokens.RefreshToken
}
