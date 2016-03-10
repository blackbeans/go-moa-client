package client

import (
	"git.wemomo.com/bibi/go-moa/proxy"
	// "runtime"
	"git.wemomo.com/bibi/go-moa/core"
	"testing"
)

func init() {

	demo := Demo{make(map[string][]string, 2), "/service/lookup"}
	inter := (*IHello)(nil)
	uinter := (*IUserService)(nil)
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
				Instance:   UserServiceDemo{},
				Interface:  uinter},
			proxy.Service{
				ServiceUri: "/service/user-service-panic",
				Instance:   UserServicePanic{},
				Interface:  uinter}}
	})

}

func TestMakeRpcFunc(t *testing.T) {

	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}},
			proxy.Service{
				ServiceUri: "/service/user-service-panic",
				Interface:  &UserService{}}})
	h := consumer.GetService("/service/user-service").(*UserService)
	a, err := h.GetName("a")
	t.Logf("--------Hello,Buddy|%s|%s\n", a, err)
	if nil != err || a.Uri != "/service/user-service" {
		t.Fail()
	}
	//---------no return
	h.SetName("a")
	//----no args
	err = h.Ping()
	t.Logf("--------Ping|%s\n", err)
	if nil != err {
		t.Fail()
	}

	_, err = h.Pong()
	t.Logf("--------Pong|%s\n", err)
	if nil != err {
		t.Fail()
	}

	h = consumer.GetService("/service/user-service-panic").(*UserService)
	a, err = h.GetName("a")
	t.Logf("--------Hello,Buddy|%s|%s\n", a, err)
	if nil == err || nil != a {
		t.Fail()
	}
	//---------no return
	h.SetName("a")

}

func BenchmarkParallerMakeRpcFunc(b *testing.B) {
	b.StopTimer()
	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}}})
	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h := consumer.GetService("/service/user-service").(*UserService)
			a, _ := h.GetName("a")
			if a.Uri != "/service/user-service" {
				b.Fail()
			}
		}
	})

}

func BenchmarkMakeRpcFunc(b *testing.B) {

	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}}})
	for i := 0; i < b.N; i++ {
		h := consumer.GetService("/service/user-service").(*UserService)
		a, _ := h.GetName("a")
		b.Logf("--------Hello,Buddy|%s\n", a)
		if a.Uri != "/service/user-service" {
			b.Fail()
		}
	}
}
