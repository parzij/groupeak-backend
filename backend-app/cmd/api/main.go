package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"groupeak/internal/config"
	"groupeak/internal/db"
	"groupeak/internal/eventbus"
	"groupeak/internal/handlers"
	"groupeak/internal/repository"
	"groupeak/internal/router"
	"groupeak/internal/services"
	workers "groupeak/internal/worker"

	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// @title Groupeak API
// @version 1.0
// @description REST API бэкенда для мобильного приложения Groupeak (управление проектами и задачами).
// @host localhost:3030
// @BasePath /api/v1
//
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Введи токен в формате: Bearer {твой_jwt_токен}
func main() {
	// Загружаем переменные окружения
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using OS env only")
	}

	cfg := config.Load()

	// 1. База данных
	sqlDB, err := db.NewPostgres(cfg.DBDsn, "./migrations")
	if err != nil {
		log.Fatalf("connect to db / run migrations: %v", err)
	}
	defer sqlDB.Close()

	// 2. Логгер и S3
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	minioClient, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Fatalf("failed to init minio client: %v", err)
	}

	// 3. Шина событий (EventBus)
	bus := eventbus.NewEventBus(logger)

	// 4. Репозитории
	taskRepo := repository.NewTaskRepository()
	projectRepo := repository.NewProjectRepository()
	userRepo := repository.NewUserRepository()
	eventRepo := repository.NewEventRepository()
	notifRepo := repository.NewNotificationRepository()

	// 5. Сервисы
	authService := services.NewAuthService(sqlDB, userRepo, cfg.JWTToken, logger)
	taskService := services.NewTaskService(sqlDB, taskRepo, logger, bus)
	projectService := services.NewProjectService(sqlDB, projectRepo, logger, bus)
	eventService := services.NewEventService(sqlDB, eventRepo, logger, bus)
	notifService := services.NewNotificationService(sqlDB, notifRepo, taskRepo, eventRepo, logger)

	// 6. Настройка и запуск EventBus
	bus.SetHandler(notifService.HandleEvent)

	// Контекст для фоновых задач (Bus и Workers)
	appCtx, cancel := context.WithCancel(context.Background())

	defer bus.Stop() // 2. Подождет завершения воркеров
	defer cancel()   // 1. Моментально пошлет сигнал выключения всем фоновым задачам

	bus.Start(appCtx)

	// 7. Инициализация и запуск фонового воркера (Напоминания 24ч)
	deadlineWorker := workers.NewDeadlineWorker(sqlDB, bus, logger)
	deadlineWorker.Start(appCtx)

	// 8. Хендлеры
	authHandler := handlers.NewAuthHandler(authService, minioClient, cfg.S3Bucket, cfg.S3Endpoint)
	projectHandler := handlers.NewProjectHandler(projectService)
	taskHandler := handlers.NewTaskHandler(taskService)
	eventHandler := handlers.NewEventHandler(eventService)
	notificationHandler := handlers.NewNotificationHandler(notifService)

	// 9. Роутер
	r := router.NewRouter(
		authHandler,
		projectHandler,
		taskHandler,
		eventHandler,
		notificationHandler,
		[]byte(cfg.JWTToken),
	)

	// 10. Запуск сервера с поддержкой Graceful Shutdown
	server := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: r,
	}

	go func() {
		log.Printf("starting HTTP server on %s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Ожидаем сигнала прерывания
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down server...")

	// Даем серверу 5 секунд на завершение текущих запросов
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal("server forced to shutdown:", err)
	}

	log.Println("server exited")
}
