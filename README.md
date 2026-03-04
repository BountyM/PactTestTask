# Telegram Service
Сервис на Go для управления несколькими независимыми подключениями к Telegram через библиотеку gotd/td.
Предоставляет gRPC API для создания/удаления соединений, авторизации по QR-коду, отправки и получения текстовых сообщений.

Динамическое создание и удаление соединений (каждое соединение изолировано).
Авторизация через QR-код (Settings → Devices → Scan QR).
Отправка текстовых сообщений через любое активное соединение.
Получение входящих сообщений для конкретного соединения.
Полная изоляция: сбой в одном соединении не влияет на остальные.
Хранение состояния в памяти (не требует внешних БД).
Конфигурация через переменные окружения.

## Требования
Go 1.26 (последняя версия)
Make (для сборки через make all)
Telegram API ID и Hash (получить можно на my.telegram.org/apps)

## Запуск
make all
создаст .env копию .env.example в ./internal/config
поменяйте в .env TELEGRAM_APP_ID TELEGRAM_APP_HASH на свои значения

## Ручки
### Создать соединение и получить QR
grpcurl -plaintext localhost:50051 pact.telegram.TelegramService.CreateSession
ответ
{
    "session_id": "session_id",
    "qr_code": "qr"
}

### Отправить сообщение
grpcurl \
	-plaintext \
	-d '{"peer":"@durov","session_id":"session_id","text":"тест"}' \
	'localhost:50051' \
	pact.telegram.TelegramService.SendMessage

### Удалить соединение
grpcurl \
	-plaintext \
	-d '{"session_id":"session_id"}' \
	'localhost:50051' \
	pact.telegram.TelegramService.DeleteSession

### Получить сообщения
grpcurl \
	-plaintext \
	-d '{"session_id":"session_id"}' \
	'localhost:50051' \
	pact.telegram.TelegramService.SubscribeMessages
