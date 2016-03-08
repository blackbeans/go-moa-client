package client

import (
	// "runtime"
	"errors"
	"fmt"
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
}

type IHello interface {
	GetService(serviceUri, proto string) (DemoResult, error)
	// 注册
	RegisterService(serviceUri, hostPort, proto string, config map[string]string) (string, error)
	// 注销
	UnregisterService(serviceUri, hostPort, proto string, config map[string]string) (string, error)
}

type DemoParam struct {
	Name string
}

type Demo struct {
	hosts map[string][]string
	uri   string
}

func (self Demo) GetService(serviceUri, proto string) (DemoResult, error) {
	result := DemoResult{}
	val, _ := self.hosts[serviceUri+"_"+proto]
	result.Hosts = val
	result.Uri = serviceUri
	return result, nil
}

// 注册
func (self Demo) RegisterService(serviceUri, hostPort, proto string, config map[string]string) (string, error) {
	self.hosts[serviceUri+"_"+proto] = []string{hostPort + "?timeout=1000&version=2"}
	fmt.Println("RegisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS", nil
}

// 注销
func (self Demo) UnregisterService(serviceUri, hostPort, proto string, config map[string]string) (string, error) {
	delete(self.hosts, serviceUri+"_"+proto)
	fmt.Println("UnregisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS", nil
}

type IUserService interface {
	GetName(name string) (*DemoResult, error)
	SetName(name string) error
	Ping() error
	Pong() (string, error)
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
	return "", nil
}

type UserServicePanic struct{}

func (self UserServicePanic) GetName(name string) (*DemoResult, error) {
	panic(errors.New("GetName invoke fail"))
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
