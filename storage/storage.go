package storage

import (
	"encoding/json"
	"go-task-manager/models"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

type TaskStorage struct {
	tasks  map[int]models.Task
	nextID int
	mu     sync.Mutex
	ops    chan func(map[int]models.Task, *int)
}

func NewTaskStorage() *TaskStorage {
	ts := &TaskStorage{
		tasks:  make(map[int]models.Task),
		nextID: 1,
		ops:    make(chan func(map[int]models.Task, *int)),
	}
	ts.loadTasks()
	go ts.run()
	return ts
}

func (ts *TaskStorage) run() {
	for op := range ts.ops {
		ts.mu.Lock()
		op(ts.tasks, &ts.nextID)
		ts.mu.Unlock()
	}
}

func (ts *TaskStorage) CreateTask(task models.Task) models.Task {
	result := make(chan models.Task)
	ts.ops <- func(tasks map[int]models.Task, nextID *int) {
		task.ID = *nextID
		tasks[*nextID] = task
		*nextID++
		ts.saveTasks()
		result <- task
	}
	return <-result
}

func (ts *TaskStorage) GetTask(id int) (models.Task, bool) {
	result := make(chan models.Task)
	exists := make(chan bool)
	ts.ops <- func(tasks map[int]models.Task, nextID *int) {
		task, found := tasks[id]
		result <- task
		exists <- found
	}
	return <-result, <-exists
}

func (ts *TaskStorage) UpdateTask(id int, updatedTask models.Task) (models.Task, bool) {
	result := make(chan models.Task)
	exists := make(chan bool)
	ts.ops <- func(tasks map[int]models.Task, nextID *int) {
		_, found := tasks[id]
		if found {
			updatedTask.ID = id
			tasks[id] = updatedTask
			ts.saveTasks()
		}
		result <- updatedTask
		exists <- found
	}
	return <-result, <-exists
}

func (ts *TaskStorage) DeleteTask(id int) bool {
	result := make(chan bool)
	ts.ops <- func(tasks map[int]models.Task, nextID *int) {
		if _, found := tasks[id]; found {
			delete(tasks, id)
			ts.saveTasks()
			result <- true
		} else {
			result <- false
		}
	}
	return <-result
}

func (ts *TaskStorage) GetAllTasks() []models.Task {
	result := make(chan []models.Task)
	ts.ops <- func(tasks map[int]models.Task, nextID *int) {
		allTasks := make([]models.Task, 0, len(tasks))
		for _, task := range tasks {
			allTasks = append(allTasks, task)
		}
		result <- allTasks
	}
	return <-result
}

func (ts *TaskStorage) saveTasks() {
	file, err := json.MarshalIndent(ts.tasks, "", " ")
	if err != nil {
		log.Println("Error marshalling tasks:", err)
		return
	}
	err = ioutil.WriteFile("tasks.json", file, 0644)
	if err != nil {
		log.Println("Error writing tasks to file::", err)
	}
}

func (ts *TaskStorage) loadTasks() {
	if _, err := os.Stat("tasks.json"); os.IsNotExist(err) {
		return
	}
	file, err := ioutil.ReadFile("tasks.json")
	if err != nil {
		log.Println("Error reading tasks file:", err)
		return
	}
	err = json.Unmarshal(file, &ts.tasks)
	if err != nil {
		log.Println("Error unmarshalling tasks:", err)
		return
	}
	for id := range ts.tasks {
		if id >= ts.nextID {
			ts.nextID = id + 1
		}
	}
}
