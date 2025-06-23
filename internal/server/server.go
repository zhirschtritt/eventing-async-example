package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zhirschtritt/eventing/internal/domain"
	"github.com/zhirschtritt/eventing/internal/events"
	"github.com/zhirschtritt/eventing/internal/migrations"
	"github.com/zhirschtritt/eventing/internal/repository"
)

type Config struct {
	Port              string `env:"PORT" envDefault:"8080"`
	DBConnString      string `env:"DB_CONN_STRING" envDefault:"postgres://postgres:postgres@localhost:5444/eventing_wal?sslmode=disable"`
	EventConsumerType string `env:"EVENT_CONSUMER_TYPE" envDefault:"gochannel"`
}

type Server struct {
	logger        *slog.Logger
	startTime     time.Time
	db            *pgxpool.Pool
	config        *Config
	migrator      *migrations.Migrator
	userService   *domain.UserService
	eventConsumer events.EventConsumer
	*http.Server
}

type HealthResponse struct {
	Status    string        `json:"status"`
	Uptime    time.Duration `json:"uptime"`
	StartTime time.Time     `json:"start_time"`
}

func NewServer() *Server {
	startTime := time.Now()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Parse configuration from environment variables
	config := &Config{}
	if err := env.Parse(config); err != nil {
		logger.Error("failed to parse configuration", "error", err)
		os.Exit(1)
	}

	logger.Info("configuration loaded", "port", config.Port, "db_conn_string", config.DBConnString, "event_consumer_type", config.EventConsumerType)

	server := &Server{
		logger:    logger,
		startTime: startTime,
		config:    config,
	}

	if err := server.initDatabase(); err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	if err := server.initEventConsumer(); err != nil {
		logger.Error("failed to initialize event consumer", "error", err)
		os.Exit(1)
	}

	if err := server.initUserService(); err != nil {
		logger.Error("failed to initialize user service", "error", err)
		os.Exit(1)
	}

	router := chi.NewRouter()

	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", config.Port),
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	server.Server = httpServer
	server.setupRoutes(router)

	return server
}

func (s *Server) setupRoutes(router *chi.Mux) {
	router.Get("/healthz", s.healthHandler)

	userRouter := NewUserRouter(s.userService, s.logger)
	router.Mount("/users", userRouter.Routes())
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)

	response := HealthResponse{
		Status:    "healthy",
		Uptime:    uptime,
		StartTime: s.startTime,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode health response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) Start() error {
	s.logger.Info("starting server", "port", s.config.Port, "start_time", s.startTime)

	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("could not listen on", "addr", s.Addr, "error", err)
		}
	}()

	s.logger.Info("server is ready to handle requests", "addr", s.Addr)

	s.gracefulShutdown()

	return nil
}

func (s *Server) initDatabase() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connString := s.config.DBConnString

	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return fmt.Errorf("failed to parse connection string: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	s.logger.Info("connecting to database", "conn_string", connString)

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	s.logger.Info("database connection established successfully")
	s.db = pool

	migrator, err := migrations.NewMigrator(connString, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	s.migrator = migrator

	if err := migrator.Up(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func (s *Server) initEventConsumer() error {
	ctx := context.Background()

	if s.config.EventConsumerType == "gochannel" {
		consumer := events.NewConsumer(
			repository.NewDBEventsRepository(s.db),
			events.ConsumerOptions{
				BufferSize:   1000,
				BatchSize:    100,
				BatchTimeout: 100 * time.Millisecond,
				WorkerCount:  4,
			})

		s.eventConsumer = consumer
		s.eventConsumer.Start(ctx)
		s.logger.Info("initialized GoChannel consumer")
	} else if s.config.EventConsumerType == "wal" {
		walConsumer, err := events.NewWALConsumer(
			repository.NewDBEventsRepository(s.db),
			events.WALConsumerOptions{
				BufferSize:       1000,
				BatchSize:        100,
				BatchTimeout:     100 * time.Millisecond,
				WALDir:           "./wal",
				WALPrefix:        "event_",
				SegmentThreshold: 1000,
				MaxSegments:      10,
				IsInSyncDiskMode: false,
				WorkerCount:      4,
			})
		if err != nil {
			return fmt.Errorf("failed to create WAL consumer: %w", err)
		}

		s.eventConsumer = walConsumer
		s.eventConsumer.Start(ctx)
		s.logger.Info("initialized WAL consumer")
	} else {
		return fmt.Errorf("unsupported event consumer type: %s", s.config.EventConsumerType)
	}

	return nil
}

func (s *Server) initUserService() error {
	userRepo := repository.NewDBUserRepository(s.db)
	s.userService = domain.NewUserService(userRepo, s.eventConsumer)
	return nil
}

func (s *Server) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	sig := <-quit
	s.logger.Info("server is shutting down", "reason", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.SetKeepAlivesEnabled(false)
	if err := s.Shutdown(ctx); err != nil {
		s.logger.Error("could not gracefully shutdown the server", "error", err)
	}

	if s.migrator != nil {
		if err := s.migrator.Close(); err != nil {
			s.logger.Error("could not close migrator", "error", err)
		}
		s.logger.Info("migrator closed")
	}

	if s.eventConsumer != nil {
		s.eventConsumer.Stop()
		s.logger.Info("event consumer stopped")
	}

	if s.db != nil {
		s.db.Close()
		s.logger.Info("database connection closed")
	}

	s.logger.Info("server stopped")
}

func startServer() error {
	server := NewServer()
	return server.Start()
}
