package client

import (
	"testing"
	"time"

	"github.com/blackbeans/go-moa/core"
	"github.com/blackbeans/go-moa/lb"
)

var consumer *MoaConsumer

func init() {

	uinter := (*IUserService)(nil)
	core.NewApplcation("../conf/moa.toml", func() []core.Service {
		return []core.Service{
			core.Service{
				ServiceUri: "/service/user-service",
				Instance:   UserServiceDemo{},
				Interface:  uinter},
			core.Service{
				ServiceUri: "/service/user-service-panic",
				Instance:   UserServicePanic{},
				Interface:  uinter,
				GroupId:    "s-mts-group"}}
	})

}

func TestNoGroupMakeRpcFunc(t *testing.T) {

	//等待5s注册地址
	time.Sleep(5 * time.Second)

	consumer := NewMoaConsumer("../conf/moa.toml",
		[]Service{
			Service{
				ServiceUri: "/service/user-service",
				Interface:  &UserService{}},
			Service{
				ServiceUri: "/service/user-service-panic",
				Interface:  &UserServicePanic{},
				GroupIds:   []string{"s-mts-group"}}})
	time.Sleep(10 * time.Second)
	s, _ := consumer.GetService("/service/user-service")
	h := s.(*UserService)
	a, err := h.GetName("a")

	if nil != err {
		t.Logf("--------Hello,Buddy|No Clients|%s\n", err)
		t.FailNow()
	} else {
		t.Logf("Oops--------Hello,Buddy|Has Clients|%s|%v\n", a, err)
	}


	time := time.Unix(time.Now().UnixNano()/1000,0)
	err = h.GetTime(time)
	if nil ==err {
		t.Logf("--------Should Error But not|%s\n", time)
		t.FailNow()
	}

	sp, _ := consumer.GetServiceWithGroupid("/service/user-service-panic", "s-mts-group")
	panicService := sp.(*UserServicePanic)
	a, err = panicService.GetName("a")
	t.Logf("--------Hello,Buddy|%s|error(%s)\n", a, err)
	if nil == err || nil != a {
		t.Fail()
	}
	//---------no return
	panicService.SetName("a")
	consumer.Destroy()
}

func BenchmarkMakeRpcFuncParallel(b *testing.B) {
	b.StopTimer()
	consumer := NewMoaConsumer("../conf/moa.toml",
		[]Service{Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}}})
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
	consumer := NewMoaConsumer("../conf/moa.toml",
		[]Service{Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}}})

	time.Sleep(10 * time.Second)
	ips, ok := consumer.clientManager.uri2Ips["/service/user-service"]
	if !ok {
		t.FailNow()
	}
	exist := false
	ips.Iterator(func(idx int, hp string) {
		if hp == "10.0.1.181:13000" {
			exist = true
		}
	})
	if !exist {
		t.Fail()
		t.Logf("RegisteService|SUCC|But Client Not Get|%s", "10.0.1.181:13000")
		return
	}

	t.Log("-----------Remove Node 10.0.1.181:13000")

	succ := consumer.clientManager.addrManager.registry.UnRegisteService("/service/user-service", "10.0.1.181:13000", lb.PROTOCOL, "")
	if !succ {
		t.Fail()
		return
	}
	time.Sleep(15 * time.Second)
	ips, ok = consumer.clientManager.uri2Ips["/service/user-service"]
	exist = false
	ips.Iterator(func(idx int, hp string) {
		if hp == "10.0.1.181:13000" {
			exist = true
		}
	})

	if exist {
		t.FailNow()
	}
	consumer.Destroy()
}
