package main

import (
	"fmt"
	"net"

	"github.com/BountyM/PactTestTask/internal/config"
	"github.com/BountyM/PactTestTask/internal/logger"
	"github.com/BountyM/PactTestTask/internal/service"
	pb "github.com/BountyM/PactTestTask/proto"
	"google.golang.org/grpc"
)

func main() {

	cfg, err := config.Load()
	if err != nil {
		return
	}

	log := logger.New(cfg.Logger)
	log.Info("Logger initialized")

	// Создание сервиса
	telegramService := service.NewTelegramService(
		&cfg.TelegramApp,
		log,
	)

	// Настройка gRPC‑сервера
	server := grpc.NewServer()

	// Запуск сервера
	lis, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Warn(fmt.Sprintf("failed to listen: %v", err))
		return
	}

	log.Info(fmt.Sprintf("gRPC server listening on :%s", cfg.GRPC.Port))

	pb.RegisterTelegramServiceServer(server, telegramService)

	if err := server.Serve(lis); err != nil {
		log.Warn(fmt.Sprintf("failed to serve: %v", err))
		return
	}
}
