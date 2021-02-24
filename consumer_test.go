package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/blackbeans/go-moa"
)

type DemoResult struct {
	Hosts []string `json:"hosts"`
	Uri   string   `json:"uri"`
}

type UserService struct {
	GetName func(ctx context.Context, serviceUri string) (*DemoResult, error)
	SetName func(name string) error
	Ping    func() error
	Pong    func() (string, error)
	GetTime func(t time.Time) error
}

type IUserService interface {
	GetName(ctx context.Context, name string) (*DemoResult, error)
	SetName(name string) error
	Ping() error
	Pong() (string, error)
	GetTime(t time.Time) error
}

type UserServiceDemo struct{}

func (self UserServiceDemo) GetName(ctx context.Context, name string) (*DemoResult, error) {

	//properties, ok := core.GetMoaProperty(ctx,"Accept-Language")
	//if ok {
	//fmt.Printf("-------------properties:%v\n",properties)
	//}

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

func (self UserServiceDemo) GetTime(t time.Time) error {
	return nil
}

type UserServicePanic struct{}

func (self UserServicePanic) GetName(ctx context.Context, name string) (*DemoResult, error) {
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

func (self UserServicePanic) GetTime(t time.Time) error {
	return nil
}

func initMoaServer() *core.Application {

	uinter := (*IUserService)(nil)
	return core.NewApplication("./benchmark/conf/moa.toml", func() []core.Service {
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
	app := initMoaServer()
	defer app.DestroyApplication()
	//等待5s注册地址
	time.Sleep(10 * time.Second)

	consumer := NewMoaConsumer("./benchmark/conf/moa.toml",
		[]Service{
			Service{
				ServiceUri: "/service/user-service",
				Interface:  &UserService{}},
			Service{
				ServiceUri: "/service/user-service-panic",
				Interface:  &UserServicePanic{},
				GroupIds:   []string{"s-mts-group"}}})

	s, _ := consumer.GetService("/service/user-service")
	h := s.(*UserService)

	ctx := core.AttachMoaProperty(context.Background(), "Accept-Language", "zh-CN")
	a, err := h.GetName(ctx, "a")
	if nil != err {
		t.Logf("--------Hello,Buddy|No Clients|%s\n", err)
		t.FailNow()
	} else {
		t.Logf("Oops--------Hello,Buddy|Has Clients|%s|%v\n", a, err)
	}

	time := time.Unix(time.Now().UnixNano()/1000, 0)
	err = h.GetTime(time)
	if nil == err {
		t.Logf("--------Should Error But not|%s\n", time)
		t.FailNow()
	}

	sp, _ := consumer.GetServiceWithGroupid("/service/user-service-panic", "s-mts-group")
	panicService := sp.(*UserServicePanic)
	a, err = panicService.GetName(context.TODO(), "a")
	t.Logf("--------Hello,Buddy|%s|error(%s)\n", a, err)
	if nil == err || nil != a {
		t.Fail()
	}
	//---------no return
	panicService.SetName("a")
	consumer.Destroy()
}

func TestClientChange(t *testing.T) {

	ipaddress := ""
	inters, err := net.Interfaces()
	if nil != err {
		panic(err)
	} else {
	outter:
		for _, inter := range inters {
			addrs, _ := inter.Addrs()
			for _, addr := range addrs {
				if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() {
					if nil != ip.IP.To4() {
						ipaddress = ip.IP.To4().String()
						break outter
					}
				}
			}
		}

	}

	if len(ipaddress) <= 0 {
		panic("ip not found")
	}

	app := initMoaServer()
	defer app.DestroyApplication()
	time.Sleep(10 * time.Second)
	consumer := NewMoaConsumer("./benchmark/conf/moa.toml",
		[]Service{Service{
			ServiceUri: "/service/user-service",
			Interface:  &UserService{}}})

	time.Sleep(10 * time.Second)
	ips, ok := consumer.clientManager.onlineUri2Ips.Load("/service/user-service")
	if !ok {
		t.FailNow()
	}
	fmt.Printf("/service/user-service----------%v\n", ips)
	exist := false
	ips.(Strategy).Iterator(func(idx int, hp core.ServiceMeta) {
		if hp.HostPort == ipaddress+":13000" {
			exist = true
		}
	})
	if !exist {
		t.Fail()
		t.Logf("RegisteService|SUCC|But Client Not Get|%s", ipaddress+":13000")
		return
	}

	t.Log("-----------Remove Node " + ipaddress + ":13000")

	succ := consumer.clientManager.addrManager.registry.UnRegisteService("/service/user-service",
		ipaddress+":13000", core.PROTOCOL, "")
	if !succ {
		t.Fail()
		return
	}
	time.Sleep(15 * time.Second)
	ips, ok = consumer.clientManager.onlineUri2Ips.Load("/service/user-service")
	exist = false
	ips.(Strategy).Iterator(func(idx int, hp core.ServiceMeta) {
		if hp.HostPort == ipaddress+":13000" {
			exist = true
		}
	})

	if exist {
		t.FailNow()
	}
	consumer.Destroy()
	t.Log("TestClientChange FINISH ")
}
