package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	charset    = []rune("0123456789abcdefghijklmnopqrstuvwxyz")
	numWorkers = 16
)

func main() {
	bot, err := tgbotapi.NewBotAPI("TOKEN")
	if err != nil {
		panic(err)
	}

	bot.Debug = true
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if !update.Message.IsCommand() {
			continue
		}

		switch update.Message.Command() {
		case "start":
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет! Отправь команду /crack и пароль для подбора")
			bot.Send(msg)
		case "crack":
			args := update.Message.CommandArguments()
			if len(args) == 0 {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Пожалуйста, укажи пароль после команды /crack")
				bot.Send(msg)
				continue
			}

			pass := args
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Начинаю подбор пароля: %s...", pass))
			bot.Send(msg)

			go func(chatID int64, password string) {
				result := crackPassword(password)
				msg := tgbotapi.NewMessage(chatID, result)
				bot.Send(msg)
			}(update.Message.Chat.ID, pass)
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Неизвестная команда")
			bot.Send(msg)
		}
	}
}

func crackPassword(pass string) string {
	start := time.Now()
	length := len(pass)
	total := pow(len(charset), length)

	var found atomic.Value
	var wg sync.WaitGroup
	var stopFlag int32 = 0

	wg.Add(1)
	go func() {
		defer wg.Done()
		file, err := os.Open("database.json")
		if err != nil {
			fmt.Println("Ошибка открытия файла c популярными паролями:", err)
			return
		}
		defer file.Close()

		bytes, err := io.ReadAll(file)
		if err != nil {
			fmt.Println("Ошибка чтения файла c популярными паролями:", err)
			return
		}

		var poppass []string
		err = json.Unmarshal(bytes, &poppass)
		if err != nil {
			fmt.Println("Ошибка парсинга JSON:", err)
			return
		}

		for _, p := range poppass {
			if p == pass {
				found.Store(pass)
				atomic.StoreInt32(&stopFlag, 1)
				return
			}
		}
	}()

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID, total int) {
			defer wg.Done()
			for i := workerID; i < total; i += numWorkers {
				if atomic.LoadInt32(&stopFlag) == 1 {
					return
				}
				guess := indexToString(i, length)
				if guess == pass {
					found.Store(guess)
					atomic.StoreInt32(&stopFlag, 1)
					return
				}
			}
		}(w, total)
	}

	wg.Wait()

	if value := found.Load(); value != nil {
		return fmt.Sprintf("Пароль найден: %s\nВремя подбора: %v", value.(string), time.Since(start))
	}
	return "Пароль не найден"
}

func indexToString(index int, length int) string {
	base := len(charset)
	result := make([]rune, length)
	for i := length - 1; i >= 0; i-- {
		result[i] = charset[index%base]
		index /= base
	}
	return string(result)
}

func pow(base, exp int) int {
	result := 1
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}
