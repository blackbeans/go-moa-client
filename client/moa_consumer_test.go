package client

import (
	"github.com/blackbeans/go-moa/proxy"
	// "runtime"

	"github.com/blackbeans/go-moa/core"
	// "sync"
	"testing"
	"time"
)

var consumer *MoaConsumer

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

	//等待5s注册地址
	time.Sleep(1 * time.Second)

	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}},
			proxy.Service{
				ServiceUri: "/service/user-service-panic",
				Interface:  &UserService{}}})
	time.Sleep(2 * time.Second)
	h := consumer.GetService("/service/user-service").(*UserService)
	a, err := h.GetName("a")
	t.Logf("--------Hello,Buddy|%s|%s\n", a, err)
	if nil != err || a.Uri != "/service/user-service" {
		t.Fail()
	}

	// ---------no return
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
	t.Logf("--------Hello,Buddy|%s|error(%s)\n", a, err)
	if nil == err || nil != a {
		t.Fail()
	}
	//---------no return
	h.SetName("a")

	// 暂停一下，不然moa-stat统计打印不出来
	time.Sleep(time.Second * 2)

	consumer.Destory()

}

func BenchmarkParallerMakeRpcFunc(b *testing.B) {
	b.StopTimer()
	consumer = NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}}})
	time.Sleep(5 * time.Second)
	h := consumer.GetService("/service/user-service").(*UserService)
	b.StartTimer()

	b.RunParallel(func(pb *testing.PB) {

		for pb.Next() {

			a, err := h.GetName("a")

			if nil != err || a.Uri != "/service/user-service" {
				b.Fail()
			}
		}
	})
	b.StopTimer()
	consumer.Destory()
}

func TestClientChange(t *testing.T) {
	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}}})

	succ := consumer.clientManager.addrManager.registry.RegisteService("/service/user-service", "127.0.0.3:1300", "redis")
	if !succ {
		t.Fail()
		return
	}
	t.Logf("RegisteService|SUCC|%s", "127.0.0.3:1300")
	time.Sleep(5 * time.Second)

	_, ok := consumer.clientManager.ip2Client["127.0.0.3:1300"]
	if !ok {
		t.Fail()
		t.Logf("RegisteService|SUCC|But Client Not Get|%s", "127.0.0.3:1300")
		return
	}

	t.Log("-----------Remove Node 127.0.0.3:1300")

	succ = consumer.clientManager.addrManager.registry.UnRegisteService("/service/user-service", "127.0.0.3:1300", "redis")
	if !succ {
		t.Fail()
		return
	}
	time.Sleep(10 * time.Second)
	_, ok = consumer.clientManager.ip2Client["127.0.0.3:1300"]
	if ok {
		t.Fail()
		t.Logf("UnRegisteService|SUCC|But Client  Get It |%s", "127.0.0.3:1300")
		return
	}

	consumer.Destory()
}

func BenchmarkMakeRpcFunc(b *testing.B) {

	b.StopTimer()
	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}}})
	time.Sleep(5 * time.Second)
	h := consumer.GetService("/service/user-service").(*UserService)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		h.GetName("a")
		// a, _ := h.GetName("a")
		// b.Logf("--------Hello,Buddy|%s\n", a)
		// if a.Uri != "/service/user-service" {
		// 	b.Fail()
		// }
	}
	b.StopTimer()
	consumer.Destory()
}
