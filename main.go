// main.go
package main

import (
	"context"
	"encoding/json"
	"github.com/dgrijalva/jwt-go"
	"go-task-manager/models"
	"go-task-manager/storage"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"strconv"
	"time"
)

var taskStorage *storage.TaskStorage
var userStorage *storage.UserStorage
var clients = make(map[chan models.Task]bool)

var jwtKey = []byte("my_secret_key")

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

func main() {
	taskStorage = storage.NewTaskStorage()
	userStorage = storage.NewUserStorage()

	http.Handle("/tasks", jwtMiddleware(http.HandlerFunc(handleTasks)))
	http.Handle("/tasks/", jwtMiddleware(http.HandlerFunc(handleTaskByID)))
	http.HandleFunc("/signup", handleSignup)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/events", handleEvents)
	http.Handle("/", http.FileServer(http.Dir("./static/")))

	log.Println("Server started at port :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleSignup(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}
	user.Password = string(hashedPassword)
	createdUser := userStorage.CreateUser(user)
	json.NewEncoder(w).Encode(createdUser)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds models.User
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := userStorage.GetUserByUsername(creds.Username)
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(creds.Password)); err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	expirationTime := time.Now().Add(5 * time.Minute)
	claims := Claims{
		Username: creds.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(`jwtkey`)
	if err != nil {
		http.Error(w, "Error creating JWT", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   tokenString,
		Expires: expirationTime,
	})
}

func jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("token")
		if err != nil {
			if err == http.ErrNoCookie {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		tknStr := c.Value
		claims := &Claims{}
		tkn, err := jwt.ParseWithClaims(tknStr, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})
		if err != nil {
			if err == jwt.ErrSignatureInvalid {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		if !tkn.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), "username", claims.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func handleTasks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		tasks := taskStorage.GetAllTasks()
		select {
		case <-ctx.Done():
			http.Error(w, "Request cancelled", http.StatusRequestTimeout)
			return
		default:
			json.NewEncoder(w).Encode(tasks)
		}
	case http.MethodPost:
		var task models.Task
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		createdTask := taskStorage.CreateTask(task)
		select {
		case <-ctx.Done():
			http.Error(w, "Request cancelled", http.StatusRequestTimeout)
			return
		default:
			json.NewEncoder(w).Encode(createdTask)
			for client := range clients {
				client <- createdTask
			}
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleTaskByID(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	id, err := strconv.Atoi(r.URL.Path[len("/tasks/"):])
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, found := taskStorage.GetTask(id)
		select {
		case <-ctx.Done():
			http.Error(w, "Request cancelled", http.StatusRequestTimeout)
			return
		default:
			if !found {
				http.Error(w, "Task not found", http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(task)
		}
	case http.MethodPut:
		var updatedTask models.Task
		if err := json.NewDecoder(r.Body).Decode(&updatedTask); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		task, found := taskStorage.UpdateTask(id, updatedTask)
		select {
		case <-ctx.Done():
			http.Error(w, "Request cancelled", http.StatusRequestTimeout)
			return
		default:
			if !found {
				http.Error(w, "Task not found", http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(task)
		}
	case http.MethodDelete:
		deleted := taskStorage.DeleteTask(id)
		select {
		case <-ctx.Done():
			http.Error(w, "Request cancelled", http.StatusRequestTimeout)
			return
		default:
			if !deleted {
				http.Error(w, "Task not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	notify := w.(http.CloseNotifier).CloseNotify()

	client := make(chan models.Task)
	clients[client] = true

	defer func() {
		delete(clients, client)
		close(client)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case task := <-client:
			json.NewEncoder(w).Encode(task)
			flusher.Flush()
		case <-notify:
			return
		}
	}
}
