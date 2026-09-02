package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	if resp.StatusCode != 201 {
		log.Println(resp.Body)
		return
	}
	id := accountID(resp)

	//accounts/id
	req, err := http.NewRequest("GET", "http://localhost:8080/accounts/"+id, nil)
	if err != nil {
		log.Println(err)
		return
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Println(resp.Body)
		return
	}
	log.Println(resp)
	//accounts/id/transactions
	req, err = http.NewRequest("GET", "http://localhost:8080/accounts/"+id+"/transactions", nil)
	if err != nil {
		log.Println(err)
		return
	}

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Println(resp.Body)
		return
	}
	log.Println(resp)
}

func accountID(r *http.Response) string {
	var data struct {
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
	}

	_ = json.NewDecoder(r.Body).Decode(&data)
	return data.Account.ID
}

func randomEmail() string {
	return fmt.Sprintf("user_%d@example.com", time.Now().Unix())
}

func randomUsername() string {
	return fmt.Sprintf("user_%d", time.Now().Unix())
}
