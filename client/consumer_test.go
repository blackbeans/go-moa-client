package client

import (
	"testing"
	"time"

	"github.com/blackbeans/go-moa/core"
)

var consumer *MoaConsumer

func init() {

	uinter := (*IUserService)(nil)
	core.NewApplcation("../conf/moa_server.toml", func() []core.Service {
		return []core.Service{
			core.Service{
				ServiceUri: "/service/user-service",
				Instance:   UserServiceDemo{},
				Interface:  uinter},
			core.Service{
				ServiceUri: "/service/user-service-panic",
				Instance:   UserServicePanic{},
				Interface:  uinter}}
	})

}

func TestNoGroupMakeRpcFunc(t *testing.T) {

	//等待5s注册地址
	time.Sleep(5 * time.Second)

	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]Service{Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}}})
	time.Sleep(5 * time.Second)
	s, _ := consumer.GetService("/service/user-service")
	h := s.(*UserService)
	a, err := h.GetName("a")

	if nil != err {
		t.Logf("--------Hello,Buddy|No Clients|%s\n", err)
		t.FailNow()
	} else {
		t.Logf("Oops--------Hello,Buddy|Has Clients|%s|%v\n", a, err)
	}
	consumer.Destroy()

}

func TestMakeRpcFunc(t *testing.T) {

	//等待5s注册地址
	time.Sleep(5 * time.Second)

	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]Service{Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{},
			GroupIds:   []string{"s-mts-group"}},
			Service{
				ServiceUri: "/service/user-service-panic",
				Interface:  &UserService{},
				GroupIds:   []string{"s-mts-group"}}})
	time.Sleep(10 * time.Second)
	s, _ := consumer.GetService("/service/user-service")
	h := s.(*UserService)
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

	pong, err := h.Pong()
	t.Logf("--------Pong|%v|%s\n", err, pong)
	if nil != err {
		t.Fail()
	}

	s, _ = consumer.GetService("/service/user-service-panic")
	h = s.(*UserService)
	a, err = h.GetName("a")
	t.Logf("--------Hello,Buddy|%s|error(%s)\n", a, err)
	if nil == err || nil != a {
		t.Fail()
	}
	//---------no return
	h.SetName("a")

	// 暂停一下，不然moa-stat统计打印不出来
	time.Sleep(time.Second * 2)

	consumer.Destroy()

}

func BenchmarkParallerMakeRpcFunc(b *testing.B) {
	b.StopTimer()
	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]Service{Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{},
			GroupIds:   []string{"s-mts-group"}}})
	time.Sleep(5 * time.Second)
	s, _ := consumer.GetService("/service/user-service")
	h := s.(*UserService)
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
	consumer.Destroy()
}

func TestClientChange(t *testing.T) {

	time.Sleep(5 * time.Second)
	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]Service{Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{},
			GroupIds:   []string{"s-mts-group"}}})

	succ := consumer.clientManager.addrManager.registry.RegisteService("/service/user-service", "127.0.0.3:1300", "redis", "s-mts-group")
	if !succ {
		t.Fail()
		return
	}
	t.Logf("RegisteService|SUCC|%s", "127.0.0.3:1300")
	time.Sleep(10 * time.Second)

	ok := consumer.clientManager.clientsManager.FindRemoteClient("127.0.0.3:1300")
	if nil == ok {
		t.Fail()
		t.Logf("RegisteService|SUCC|But Client Not Get|%s", "127.0.0.3:1300")
		return
	}

	t.Log("-----------Remove Node 127.0.0.3:1300")

	succ = consumer.clientManager.addrManager.registry.UnRegisteService("/service/user-service", "127.0.0.3:1300", "redis", "s-mts-group")
	if !succ {
		t.Fail()
		return
	}
	time.Sleep(10 * time.Second)
	ok = consumer.clientManager.clientsManager.FindRemoteClient("127.0.0.3:1300")
	if nil == ok {
		t.Fail()
		t.Logf("UnRegisteService|SUCC|But Client  Get It |%s", "127.0.0.3:1300")
		return
	}

	consumer.Destroy()
}

func BenchmarkMakeRpcFunc(b *testing.B) {

	b.StopTimer()
	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]Service{Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{},
			GroupIds:   []string{"s-mts-group"}}})
	time.Sleep(5 * time.Second)
	s, _ := consumer.GetService("/service/user-service")
	h := s.(*UserService)
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
	consumer.Destroy()
}
