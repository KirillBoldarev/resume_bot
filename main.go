package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

const telegramAPI = "https://api.telegram.org/bot"

// Telegram types
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type Message struct {
	Chat Chat `json:"chat"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type CallbackQuery struct {
	ID      string `json:"id"`
	Data    string `json:"data"`
	Message struct {
		Chat Chat `json:"chat"`
	} `json:"message"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string  `json:"text"`
	CallbackData *string `json:"callback_data,omitempty"`
	URL          *string `json:"url,omitempty"`
}

func sendJSON(method string, payload interface{}) error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN не установлен")
	}
	url := telegramAPI + token + "/" + method

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Telegram API error: %d", resp.StatusCode)
		return fmt.Errorf("статус %d", resp.StatusCode)
	}
	return nil
}

func answerCallback(id, text string) {
	req := map[string]interface{}{
		"callback_query_id": id,
		"text":              text,
	}
	sendJSON("answerCallbackQuery", req)
}

func sendFile(chatID int64, filePath string) error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN не установлен")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	_ = writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	_ = writer.WriteField("caption", "Кирилл Болдарев Frontend Developer Vuejs.pdf")

	err = writer.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", telegramAPI+token+"/sendDocument", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Ошибка отправки файла: %d", resp.StatusCode)
		return fmt.Errorf("статус %d", resp.StatusCode)
	}
	return nil
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	var update Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("Ошибка парсинга: %v", err)
		return
	}

	if update.Message != nil {
		resume := "resume"
		contacts := "contacts"
		yourID := os.Getenv("YOUR_TELEGRAM_ID")
		connectURL := "https://t.me/" + yourID

		msg := map[string]interface{}{
			"chat_id": update.Message.Chat.ID,
			"text":    "Привет! Это мой телеграмм-бот для рекрутеров:",
			"reply_markup": InlineKeyboardMarkup{
				InlineKeyboard: [][]InlineKeyboardButton{
					{{Text: "📄 Скачать резюме", CallbackData: &resume}},
					{{Text: "📞 Посмотреть контакты и соцсети", CallbackData: &contacts}},
					{{Text: "💬 Написать мне", URL: &connectURL}},
				},
			},
		}
		sendJSON("sendMessage", msg)
	}

	if update.CallbackQuery != nil {
		cq := update.CallbackQuery

		// Лоадер
		answerCallback(cq.ID, "🔄 Ожидайте, высылаю...")

		switch cq.Data {
		case "resume":
			err := sendFile(cq.Message.Chat.ID, "/app/resume.pdf")
			if err != nil {
				log.Printf("Ошибка: %v", err)
				sendJSON("sendMessage", map[string]interface{}{
					"chat_id": cq.Message.Chat.ID,
					"text":    "❌ Не удалось отправить резюме. Попробуйте позже.",
				})
			}
			// Убираем спиннер
			answerCallback(cq.ID, "")

		case "contacts":
			contactMessage := `📬 Мои контакты:
• Email: KirillBoldareb@yandex.ru
• VK: https://vk.ru/k_boldarev  
• Telegram: @kirill_boldarev
• Phone: +7 (988) 152-38-77`

			sendJSON("sendMessage", map[string]interface{}{
				"chat_id": cq.Message.Chat.ID,
				"text":    contactMessage,
			})
			answerCallback(cq.ID, "")
		}
	}

	w.WriteHeader(http.StatusOK)
}

func setWebhook() {
	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		log.Fatal("WEBHOOK_URL не установлен")
	}
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не установлен")
	}

	url := fmt.Sprintf("%s%s/setWebhook?url=%s", telegramAPI, token, webhookURL)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Не удалось установить webhook: %v", err)
	}
	defer resp.Body.Close()
	log.Println("✅ Webhook установлен:", webhookURL)
}

func main() {
	setWebhook()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/telegram-webhook", handleWebhook)
	log.Printf("🚀 Сервер запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
