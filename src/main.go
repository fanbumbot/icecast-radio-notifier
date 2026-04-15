package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
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

type Context struct {
	bot    *tgbotapi.BotAPI
	mainDB *sql.DB
	msg    map[string]string
}

func getMessage(context Context, key string) string {
	return context.msg[key]
}

func importMessages() (messages map[string]string, err error) {
	file, err := os.ReadFile("../messages.json")
	if err != nil {
		file, err = os.ReadFile("../messages_template.json")
		if err != nil {
			return
		}
	}

	var temp map[string]interface{}
	if err = json.Unmarshal(file, &temp); err != nil {
		return
	}
	messages = make(map[string]string, len(temp))
	for k, v := range temp {
		switch v := v.(type) {
		case string:
			messages[k] = v
		case []any:
			output := make([]string, len(v))
			for i, substr := range v {
				output[i] = substr.(string)
			}
			str := strings.Join(output, "\n")
			messages[k] = str
		}
	}
	return
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

func CheckStreamStatus(context Context, streamURL string, waitTime time.Duration) {
	ticker := time.NewTicker(waitTime)

	currentStatus, err := getRadioStatus(context.mainDB)
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
			err := setRadioStatus(context.mainDB, currentStatus)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			if currentStatus {
				sendMessageToAll(context.bot, context.mainDB, getMessage(context, "RADIO_ON_NOTIFICATION"))
			} else {
				sendMessageToAll(context.bot, context.mainDB, getMessage(context, "RADIO_OFF_NOTIFICATION"))
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

func sendMessageTo(bot *tgbotapi.BotAPI, chatID int64, message string) (err error) {
	msg := tgbotapi.NewMessage(chatID, message)
	_, err = bot.Send(msg)
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

func getRadioStatus(mainDB *sql.DB) (radioStatus bool, err error) {
	radioStatus = false

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
			radioStatus = value == "True"
		}
	}
	return
}

func setRadioStatus(mainDB *sql.DB, radioStatus bool) (err error) {
	var stringStatus string
	if radioStatus {
		stringStatus = "True"
	} else {
		stringStatus = "False"
	}
	_, err = mainDB.Exec("UPDATE radio_state SET value = ? WHERE key = 'site_status'", stringStatus)
	return
}

func processMessages(context Context) (err error) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := context.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if !update.Message.IsCommand() {
			continue
		}

		command := update.Message.Command()

		switch command {
		case "start":
			sendMessageTo(context.bot, update.Message.Chat.ID, getMessage(context, "START_MSG"))
		case "stop":
			sendMessageTo(context.bot, update.Message.Chat.ID, getMessage(context, "STOP_MSG"))
		case "help":
			sendMessageTo(context.bot, update.Message.Chat.ID, getMessage(context, "HELP_MSG"))
		case "status":
			status, err := getRadioStatus(context.mainDB)
			if err != nil {
				sendMessageTo(context.bot, update.Message.Chat.ID, "Unknown status")
			}
			if status {
				sendMessageTo(context.bot, update.Message.Chat.ID, getMessage(context, "RADIO_ON_INFO"))
			} else {
				sendMessageTo(context.bot, update.Message.Chat.ID, getMessage(context, "RADIO_OFF_INFO"))
			}
		case "notification_status":
			fmt.Print("Статус оповещений")
		case "radio_hist":
			fmt.Print("История эфиров")
		}

	}
	return
}

func main() {
	msg, err := importMessages()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

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

	context := Context{bot, mainDB, msg}

	go CheckStreamStatus(context, streamURL, time.Second*5)

	fmt.Println("Завершение инициализации")

	err = processMessages(context)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
}
