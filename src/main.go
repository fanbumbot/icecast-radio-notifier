package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"database/sql"

	_ "modernc.org/sqlite"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	godotenv "github.com/joho/godotenv"
)

type User struct {
	ID              int64
	IsActive        bool
	LastUserVersion int
}

type Chat struct {
	ID              int64
	IsActive        bool
	LastUserVersion int
}

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

func CheckStreamStatus(bot *tgbotapi.BotAPI, mainDB *sql.DB, streamURL string, waitTime time.Duration) {
	ticker := time.NewTicker(waitTime)

	currentStatus, err := getRadioState(mainDB)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
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
			err := setRadioState(mainDB, currentStatus)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if currentStatus {
				sendMessageToAll(bot, mainDB, "Радио запущено")
			} else {
				sendMessageToAll(bot, mainDB, "Радио остановлено")
			}
		}
		lastStatus = currentStatus
	}
}

func sendMessageToAll(bot *tgbotapi.BotAPI, mainDB *sql.DB, message string) (err error) {
	users, err := getUsers(mainDB)
	if err != nil {
		return
	}
	chats, err := getChats(mainDB)
	if err != nil {
		return
	}

	fmt.Printf("Сообщение '%v' для всех\n", message)

	for _, user := range users {
		if user.IsActive {
			msg := tgbotapi.NewMessage(user.ID, message)
			bot.Send(msg)
		}
	}
	for _, chat := range chats {
		if chat.IsActive {
			msg := tgbotapi.NewMessage(chat.ID, message)
			bot.Send(msg)
		}
	}
	return
}

func getUsers(mainDB *sql.DB) (users []*User, err error) {
	rows, err := mainDB.Query("SELECT user_id, is_active, last_used_version FROM users")
	if err != nil {
		return
	}
	defer rows.Close()

	var user User
	var isActiveInt int
	for rows.Next() {
		err = rows.Scan(&user.ID, &isActiveInt, &user.LastUserVersion)
		if err != nil {
			return
		}
		user.IsActive = isActiveInt == 1
		users = append(users, &user)
	}
	return
}

func getChats(mainDB *sql.DB) (chats []*Chat, err error) {
	rows, err := mainDB.Query("SELECT chat_id, is_active, last_used_version FROM chats")
	if err != nil {
		return
	}
	defer rows.Close()

	var chat Chat
	var isActiveInt int
	for rows.Next() {
		err = rows.Scan(&chat.ID, &isActiveInt, &chat.LastUserVersion)
		if err != nil {
			return
		}
		chat.IsActive = isActiveInt == 1
		chats = append(chats, &chat)
	}
	return
}

func getRadioState(mainDB *sql.DB) (radioState bool, err error) {
	radioState = false

	rows, err := mainDB.Query("SELECT key, value FROM radio_state")
	if err != nil {
		return
	}
	defer rows.Close()

	var key, value string
	for rows.Next() {
		err = rows.Scan(&key, &value)
		if err != nil {
			return
		}
		switch key {
		case "site_status":
			radioState = value == "True"
		}
	}
	return
}

func setRadioState(mainDB *sql.DB, radioState bool) (err error) {
	var stringState string
	if radioState {
		stringState = "True"
	} else {
		stringState = "False"
	}
	_, err = mainDB.Exec("UPDATE radio_state SET value = ? WHERE key = 'site_status'", stringState)
	return
}

func main() {
	mainDB, err := sql.Open("sqlite", "../data/database.db")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer mainDB.Close()

	journalDB, err := sql.Open("sqlite", "../data/journal.db")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer journalDB.Close()

	tokenID, streamURL, err := importEnv("../.env")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	bot, err := tgbotapi.NewBotAPI(tokenID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	go CheckStreamStatus(bot, mainDB, streamURL, time.Second*5)

	fmt.Println("Завершение инициализации")

	select {}
}
