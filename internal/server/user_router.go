package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zhirschtritt/eventing/internal/domain"
)

type UserRouter struct {
	userService *domain.UserService
	logger      *slog.Logger
}

func NewUserRouter(userService *domain.UserService, logger *slog.Logger) *UserRouter {
	return &UserRouter{
		userService: userService,
		logger:      logger,
	}
}

func (ur *UserRouter) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", ur.createUser)
	return r
}

type CreateUserResponse struct {
	User *domain.User `json:"user"`
}

func (ur *UserRouter) createUser(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ur.logger.Error("failed to decode create user request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := ur.userService.CreateUser(r.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrUserAlreadyExists) {
			ur.logger.Error("user already exists", "error", err, "email", req.Email)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if errors.Is(err, domain.ErrEventConsumerFull) {
			ur.logger.Error("event consumer is full", "error", err, "email", req.Email)
			http.Error(w, err.Error(), http.StatusTooManyRequests)
			return
		}
		ur.logger.Error("failed to create user", "error", err, "email", req.Email)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := CreateUserResponse{
		User: user,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		ur.logger.Error("failed to encode create user response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
