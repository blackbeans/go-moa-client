package client

import (
	"errors"
	"time"
)

type DemoResult struct {
	Hosts []string `json:"hosts"`
	Uri   string   `json:"uri"`
}

type UserService struct {
	GetName func(serviceUri string) (*DemoResult, error)
	SetName func(name string) error
	Ping    func() error
	Pong    func() (string, error)
	GetTime func(t time.Time) error
}

type IUserService interface {
	GetName(name string) (*DemoResult, error)
	SetName(name string) error
	Ping() error
	Pong() (string, error)
	GetTime(t time.Time) error
}

type UserServiceDemo struct{}

func (self UserServiceDemo) GetName(name string) (*DemoResult, error) {
	return &DemoResult{[]string{"a", "b"}, "/service/user-service"}, nil
}

func (self UserServiceDemo) SetName(name string) error {
	return nil
}

func (self UserServiceDemo) Ping() error {
	return nil

}
func (self UserServiceDemo) Pong() (string, error) {
	return "pong", nil
}

func(self UserServiceDemo)GetTime(t time.Time) error{
	return nil
}

type UserServicePanic struct{}

func (self UserServicePanic) GetName(name string) (*DemoResult, error) {
	return nil, errors.New("真的抛错了")
}

func (self UserServicePanic) SetName(name string) error {
	return errors.New("fuck SetName Err")
}

func (self UserServicePanic) Ping() error {
	return nil

}
func (self UserServicePanic) Pong() (string, error) {
	return "", nil
}

func(self UserServicePanic)GetTime(t time.Time) error{
	return nil
}
