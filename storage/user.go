package storage

import (
	"errors"
	"go-task-manager/models"
	"sync"
)

type UserStorage struct {
	users  map[string]models.User
	nextID int
	mu     sync.Mutex
}

func NewUserStorage() *UserStorage {
	return &UserStorage{
		users:  make(map[string]models.User),
		nextID: 1,
	}
}

func (us *UserStorage) CreateUser(user models.User) models.User {
	us.mu.Lock()
	defer us.mu.Unlock()
	user.ID = us.nextID
	us.nextID++
	us.users[user.Username] = user
	return user
}

func (us *UserStorage) GetUserByUsername(username string) (models.User, error) {
	us.mu.Lock()
	defer us.mu.Unlock()
	user, exists := us.users[username]
	if !exists {
		return models.User{}, errors.New("user not found")
	}
	return user, nil
}
