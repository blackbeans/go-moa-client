package client

import (
	"git.wemomo.com/bibi/go-moa/proxy"
	// "runtime"
	"fmt"
	"git.wemomo.com/bibi/go-moa/core"
	"reflect"
	"testing"
)

type DemoResult struct {
	Hosts []string `json:"hosts"`
	Uri   string   `json:"uri"`
}

type IHello interface {
	GetService(serviceUri, proto string) DemoResult
	// 注册
	RegisterService(serviceUri, hostPort, proto string, config map[string]string) string
	// 注销
	UnregisterService(serviceUri, hostPort, proto string, config map[string]string) string
}

type DemoParam struct {
	Name string
}

type Demo struct {
	hosts map[string][]string
	uri   string
}

func (self Demo) GetService(serviceUri, proto string) DemoResult {
	result := DemoResult{}
	val, _ := self.hosts[serviceUri+"_"+proto]
	result.Hosts = val
	result.Uri = serviceUri
	return result
}

// 注册
func (self Demo) RegisterService(serviceUri, hostPort, proto string, config map[string]string) string {
	self.hosts[serviceUri+"_"+proto] = []string{hostPort + "?timeout=1000&version=2"}
	fmt.Println("RegisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS"
}

// 注销
func (self Demo) UnregisterService(serviceUri, hostPort, proto string, config map[string]string) string {
	delete(self.hosts, serviceUri+"_"+proto)
	fmt.Println("UnregisterService|SUCC|" + serviceUri + "|" + proto)
	return "SUCCESS"
}

type IUserService interface {
	GetName(name string) *DemoResult
}

type UserServiceDemo struct{}

func (self UserServiceDemo) GetName(name string) *DemoResult {
	return &DemoResult{[]string{"a", "b"}, "/service/user-service"}
}

func init() {
	udemo := UserServiceDemo{}
	demo := Demo{make(map[string][]string, 2), "/service/lookup"}
	inter := reflect.TypeOf((*IHello)(nil)).Elem()
	uinter := reflect.TypeOf((*IUserService)(nil)).Elem()
	core.NewApplcation("../conf/moa_server.toml", func() []proxy.Service {
		return []proxy.Service{
			proxy.Service{
				ServiceUri: "/service/lookup",
				Instance:   demo,
				Interface:  inter},
			proxy.Service{
				ServiceUri: "/service/moa-admin",
				Instance:   demo,
				Interface:  inter},
			proxy.Service{
				ServiceUri: "/service/user-service",
				Instance:   udemo,
				Interface:  uinter}}
	})

	// demo.RegisterService("/service/user-service", "localhost:13000",
	// 	PROTOCOL_TYPE, make(map[string]string, 0))

}

type UserService struct {
	GetName func(serviceUri string) *DemoResult
}

func TestMakeRpcFunc(t *testing.T) {

	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/user-service",
			Instance:   &UserService{}}})
	h := consumer.GetService("/service/user-service").(*UserService)
	a := h.GetName("a")
	t.Logf("--------Hello,Buddy|%s\n", a)
	if a.Uri != "/service/user-service" {
		t.Fail()
	}

}

func BenchmarkMakeRpcFunc(b *testing.B) {

	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/user-service",
			Instance:   &UserService{}}})
	for i := 0; i < b.N; i++ {
		h := consumer.GetService("/service/user-service").(*UserService)
		a := h.GetName("a")
		b.Logf("--------Hello,Buddy|%s\n", a)
		if a.Uri != "/service/user-service" {
			b.Fail()
		}
	}

}
