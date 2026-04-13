package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	godotenv "github.com/joho/godotenv"
)

func importEnv(envPath string) (tokenID string, streamURL string, err error) {
	var exists bool

	err = godotenv.Load(envPath)
	if err != nil {
		err = errors.New(".env file does not exists")
		return
	}

	tokenID, exists = os.LookupEnv("TOKEN_ID")
	if !exists {
		err = errors.New("TOKEN_ID does not exists in environment")
		return
	}

	streamURL, exists = os.LookupEnv("STREAM_URL")
	if !exists {
		err = errors.New("STREAM_URL does not exists in environment")
		return
	}
	return
}

func CheckStreamStatus(streamURL string, ch chan bool) {
	ticker := time.NewTicker(time.Second * 5) // 5 Second

	currentStatus := false
	lastStatus := currentStatus
	for range ticker.C {
		resp, err := http.Get(streamURL)
		if err != nil {
			currentStatus = false
		} else {
			resp.Body.Close()
			currentStatus = true
		}

		if currentStatus != lastStatus {
			ch <- currentStatus
		}
		lastStatus = currentStatus
	}
}

func UpdateStreamStatus(ch chan bool) {
	for isOnline := range ch {
		if isOnline {
			fmt.Println("Радио запущено")
		} else {
			fmt.Println("Радио остановлено")
		}
	}
}

func main() {
	_, streamURL, err := importEnv("../.env")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Инициализация")

	ch := make(chan bool)
	go CheckStreamStatus(streamURL, ch)
	go UpdateStreamStatus(ch)

	select {}
}
