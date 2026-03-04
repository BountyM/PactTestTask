package service

import (
	"context"
	"fmt"
	"image/png"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/BountyM/PactTestTask/internal/config"
	"github.com/BountyM/PactTestTask/internal/models"
	"github.com/BountyM/PactTestTask/internal/utils"
	pb "github.com/BountyM/PactTestTask/proto"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"rsc.io/qr"
)

type TelegramService struct {
	cfg *config.TelegramAppConfig
	log *slog.Logger
	pb.UnimplementedTelegramServiceServer
	sessionManager *models.Manager
}

func NewTelegramService(cfg *config.TelegramAppConfig, log *slog.Logger) *TelegramService {
	return &TelegramService{
		sessionManager: models.NewManager(),
		cfg:            cfg,
		log:            log,
	}
}

func (s *TelegramService) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	logger := s.log.With(slog.String("method", "CreateSession"))
	logger.Info("Starting session creation")

	sessionID, err := utils.GenerateSessionID()
	if err != nil {
		logger.Error("Failed to generate session ID", slog.Any("error", err))
		return nil, err
	}
	logger = logger.With(slog.String("session_id", sessionID))

	sessionStorage := &models.MemorySession{}

	qrChan := make(chan string, 1)
	ctxSession, cancel := context.WithCancel(context.Background())

	// Создаём канал для сообщений сессии (буферизированный, чтобы не блокировать обработчик)
	messagesChan := make(chan *models.MessageUpdate, 100)

	client := telegram.NewClient(s.cfg.ID, s.cfg.Hash, telegram.Options{
		SessionStorage: sessionStorage,
		// Используем замыкание для передачи сессии в обработчик
		UpdateHandler: telegram.UpdateHandlerFunc(func(ctx context.Context, u tg.UpdatesClass) error {
			return s.handleUpdateForSession(ctx, sessionID, u)
		}),
	})

	sess := &models.Session{
		ID:       sessionID,
		Client:   client,
		Storage:  sessionStorage,
		Messages: messagesChan,
		Cancel:   cancel,
		Active:   true,
		Err:      make(chan error, 1),
	}

	s.sessionManager.Add(sessionID, sess)
	logger.Info("Session added to manager")

	go func() {
		// Закрываем канал сообщений при завершении горутины
		defer close(sess.Messages)
		logger.Debug("Starting client.Run goroutine")

		err := client.Run(ctxSession, func(runCtx context.Context) error {
			qrs, err := client.QR().Export(runCtx)
			if err != nil {
				logger.Error("QR export failed", slog.Any("error", err))
				return err
			}

			// Генерация QR-кода (как в исходном коде)
			//TODO удалить генерирует изображение
			code, err := qrs.Image(qr.M)
			if err != nil {
				logger.Error("Failed to create QR image", slog.Any("error", err))
				return fmt.Errorf("не удалось создать QR-код: %w", err)
			}

			qrFile := "telegram_login_qr.png"
			f, err := os.Create(qrFile)
			if err != nil {
				logger.Error("Failed to create QR file", slog.String("file", qrFile), slog.Any("error", err))
				return err
			}

			if err := png.Encode(f, code); err != nil {
				_ = f.Close() // не проверяем, так как уже есть ошибка
				logger.Error("Failed to encode QR to PNG", slog.Any("error", err))
				return err
			}

			if err := f.Close(); err != nil {
				logger.Error("Failed to close file after write", slog.String("file", qrFile), slog.Any("error", err))
				return fmt.Errorf("failed to close file: %w", err)
			}
			logger.Debug("QR code saved to file", slog.String("file", qrFile))

			select {
			case qrChan <- qrs.String():
				logger.Debug("QR token sent to channel")
			default:
				logger.Warn("QR channel was full, token not sent")
			}

			<-runCtx.Done()
			logger.Debug("Client run context done")
			return runCtx.Err()
		})
		if err != nil {
			logger.Error("Client run finished with error", slog.Any("error", err))
		} else {
			logger.Info("Client run finished successfully")
		}
		sess.Err <- err
	}()

	select {
	case qr := <-qrChan:
		logger.Info("Session created successfully, QR token obtained")
		return &pb.CreateSessionResponse{
			SessionId: &sessionID,
			QrCode:    &qr,
		}, nil

	case <-time.After(30 * time.Second):
		logger.Warn("Timeout waiting for QR code, removing session")
		s.sessionManager.Remove(sessionID)
		cancel()
		return nil, status.Error(codes.DeadlineExceeded, "timeout waiting for QR code")
	}
}

func (s *TelegramService) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*pb.DeleteSessionResponse, error) {

	sessionID := req.GetSessionId()
	logger := s.log.With(slog.String("method", "DeleteSession"), slog.String("session_id", sessionID))
	logger.Info("Deleting session")

	session, ok := s.sessionManager.Get(sessionID)
	if !ok {
		logger.Warn("Session not found")
		return nil, status.Errorf(codes.NotFound, "no session: %s", sessionID)
	}

	session.Active = false
	session.Cancel() // остановит client.Run => engine закроется
	s.sessionManager.Remove(sessionID)
	logger.Info("Session deleted successfully")

	return &pb.DeleteSessionResponse{}, nil
}

func (s *TelegramService) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	sessionID := req.GetSessionId()
	logger := s.log.With(slog.String("method", "SendMessage"), slog.String("session_id", sessionID))
	logger.Info("Sending message", slog.String("peer", req.GetPeer()), slog.String("text", req.GetText()))

	session, ok := s.sessionManager.Get(sessionID)
	if !ok || session == nil || !session.Active {
		logger.Warn("Session not found or inactive")
		return nil, status.Errorf(codes.NotFound, "no session: %s", sessionID)
	}

	api := session.Client.API()
	sender := message.NewSender(api)

	result, err := sender.Resolve(req.GetPeer()).Text(ctx, req.GetText())
	if err != nil {
		logger.Error("Failed to send message", slog.Any("error", err))
		return nil, fmt.Errorf("ошибка отправки сообщения: %w", err)
	}

	var msgID int
	switch u := result.(type) {
	case *tg.Updates:
		for _, update := range u.Updates {
			switch upd := update.(type) {
			case *tg.UpdateNewMessage:
				// Извлекаем ID из сообщения
				switch m := upd.Message.(type) {
				case *tg.Message:
					msgID = m.ID
				case *tg.MessageService:
					msgID = m.ID // сервисные сообщения тоже имеют ID
				}
			case *tg.UpdateNewChannelMessage:
				switch m := upd.Message.(type) {
				case *tg.Message:
					msgID = m.ID
				case *tg.MessageService:
					msgID = m.ID
				}
			}
		}
	}
	msgID64 := int64(msgID)
	logger.Info("Message sent successfully", slog.Int64("message_id", msgID64))

	return &pb.SendMessageResponse{MessageId: &msgID64}, nil
}

func (s *TelegramService) SubscribeMessages(req *pb.SubscribeMessagesRequest, stream pb.TelegramService_SubscribeMessagesServer) error {
	sessionID := req.GetSessionId()
	logger := s.log.With(slog.String("method", "SubscribeMessages"), slog.String("session_id", sessionID))
	logger.Info("Client subscribed to messages")
	// 1. Получаем сессию по ID
	sess, ok := s.sessionManager.Get(sessionID)
	if !ok {
		logger.Warn("Session not found")
		return status.Errorf(codes.NotFound, "session not found: %s", sessionID)
	}

	// 2. Проверяем, активна ли сессия
	active := sess.IsActive()
	if !active {
		logger.Warn("Session is not active")
		return status.Error(codes.FailedPrecondition, "session is not active")
	}

	// 3. Бесконечно читаем из канала сообщений сессии
	for {
		select {
		case <-stream.Context().Done():
			logger.Info("Client unsubscribed (context done)")
			return stream.Context().Err()

		case update, ok := <-sess.Messages:
			if !ok {
				logger.Info("Session message channel closed, ending subscription")
				return nil
			}

			// 4. Преобразуем внутреннюю структуру в protobuf-сообщение
			pbUpdate := &pb.MessageUpdate{
				MessageId: &update.MessageID,
				Text:      &update.Text,
				Timestamp: &update.Timestamp,
				From:      &update.From,
			}

			// 5. Отправляем клиенту
			if err := stream.Send(pbUpdate); err != nil {
				logger.Error("Failed to send message update to client", slog.Any("error", err))
				return err
			}
			logger.Debug("Message update sent to client",
				slog.Int64("msg_id", update.MessageID),
				slog.String("from", update.From))
		}
	}
}

// handleUpdateForSession обрабатывает обновления для конкретной сессии
func (s *TelegramService) handleUpdateForSession(ctx context.Context, sessionID string, u tg.UpdatesClass) error {
	// Получаем сессию из менеджера
	sess, ok := s.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("сессия не найдена: %w", fmt.Errorf("сессия не найдена"))
	}

	// Обработка обновлений (адаптированные функции из исходного кода)
	switch updates := u.(type) {
	case *tg.Updates:
		for _, update := range updates.Updates {
			if err := s.processUpdateForSession(ctx, sess, update); err != nil {
				log.Printf("Ошибка обработки обновления: %v", err)
			}
		}
	case *tg.UpdateShortMessage:
		return s.processShortMessageForSession(ctx, sess, updates)
	case *tg.UpdateShortChatMessage:
		return s.processShortChatMessageForSession(ctx, sess, updates)
	case *tg.UpdateShort:
		return s.processUpdateForSession(ctx, sess, updates.Update)
	case *tg.UpdatesCombined:
		for _, update := range updates.Updates {
			if err := s.processUpdateForSession(ctx, sess, update); err != nil {
				log.Printf("Ошибка обработки обновления: %v", err)
			}
		}
	default:
		// Игнорируем
	}
	return nil
}

// processUpdateForSession обрабатывает одиночное обновление
func (s *TelegramService) processUpdateForSession(ctx context.Context, sess *models.Session, update tg.UpdateClass) error {
	switch u := update.(type) {
	case *tg.UpdateNewMessage:
		return s.processMessageForSession(ctx, sess, u.Message)
	case *tg.UpdateNewChannelMessage:
		return s.processMessageForSession(ctx, sess, u.Message)
	default:
		return nil
	}
}

// processMessageForSession извлекает информацию из сообщения и отправляет в канал сессии
func (s *TelegramService) processMessageForSession(ctx context.Context, sess *models.Session, msg tg.MessageClass) error {
	message, ok := msg.(*tg.Message)
	if !ok {
		return nil
	}

	msgID := message.ID
	date := int64(message.Date)
	text := message.Message

	var senderID string

	if fromID, ok := message.GetFromID(); ok {
		switch id := fromID.(type) {
		case *tg.PeerUser:
			senderID = fmt.Sprintf("user_%d", id.UserID)

		case *tg.PeerChat:
			senderID = fmt.Sprintf("chat_%d", id.ChatID)

		case *tg.PeerChannel:
			senderID = fmt.Sprintf("channel_%d", id.ChannelID)

		default:

			senderID = "unknown"

		}
	} else {
		// Если FromID нет, проверяем PeerID для контекста
		peerID := message.GetPeerID()
		switch p := peerID.(type) {
		case *tg.PeerUser:
			senderID = fmt.Sprintf("peer_user_%d", p.UserID)

		case *tg.PeerChat:
			senderID = fmt.Sprintf("peer_chat_%d", p.ChatID)

		case *tg.PeerChannel:
			senderID = fmt.Sprintf("peer_channel_%d", p.ChannelID)

		default:
			senderID = "peer_unknown"

		}

	}

	update := &models.MessageUpdate{
		MessageID: int64(msgID),
		Text:      text,
		Timestamp: date,
		From:      senderID,
	}

	// Отправляем в канал сессии (неблокирующая отправка)
	s.sendToSessionChannel(sess, update)
	return nil
}

// processShortMessageForSession обрабатывает короткие личные сообщения
func (s *TelegramService) processShortMessageForSession(ctx context.Context, sess *models.Session, u *tg.UpdateShortMessage) error {

	update := &models.MessageUpdate{
		MessageID: int64(u.ID),
		Text:      u.Message,
		Timestamp: int64(u.Date),
		From:      fmt.Sprintf("user_%d", u.UserID),
	}
	s.sendToSessionChannel(sess, update)
	return nil
}

// processShortChatMessageForSession обрабатывает короткие групповые сообщения
func (s *TelegramService) processShortChatMessageForSession(ctx context.Context, sess *models.Session, u *tg.UpdateShortChatMessage) error {
	update := &models.MessageUpdate{
		MessageID: int64(u.ID),
		Text:      u.Message,
		Timestamp: int64(u.Date),
		From:      fmt.Sprintf("user_%d", u.FromID),
	}

	s.sendToSessionChannel(sess, update)
	return nil
}

// sendToSessionChannel отправляет обновление в канал сессии без блокировки
func (s *TelegramService) sendToSessionChannel(sess *models.Session, update *models.MessageUpdate) {
	select {
	case sess.Messages <- update:
		// успешно отправлено
	default:
		// Канал переполнен или закрыт — логируем и пропускаем
		log.Printf("Канал сообщений сессии %s переполнен, сообщение пропущено", sess.ID)
	}
}
