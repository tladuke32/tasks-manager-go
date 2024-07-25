// main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"go-task-manager/models"
	"go-task-manager/storage"
)

var taskStorage = storage.NewTaskStorage()

func createTaskHandler(w http.ResponseWriter, r *http.Request) {
	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task = taskStorage.CreateTask(task)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func getTaskHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}
	task, exists := taskStorage.GetTask(id)
	if !exists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}
	var updatedTask models.Task
	if err := json.NewDecoder(r.Body).Decode(&updatedTask); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task, exists := taskStorage.UpdateTask(id, updatedTask)
	if !exists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}
	if !taskStorage.DeleteTask(id) {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func getAllTasksHandler(w http.ResponseWriter, r *http.Request) {
	tasks := taskStorage.GetAllTasks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/tasks", getAllTasksHandler).Methods("GET")
	r.HandleFunc("/tasks", createTaskHandler).Methods("POST")
	r.HandleFunc("/tasks/{id}", getTaskHandler).Methods("GET")
	r.HandleFunc("/tasks/{id}", updateTaskHandler).Methods("PUT")
	r.HandleFunc("/tasks/{id}", deleteTaskHandler).Methods("DELETE")

	fmt.Println("Starting server at port 8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
