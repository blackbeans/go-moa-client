package main

import (
	"context"
	"flag"
	"github.com/blackbeans/go-moa"
	"github.com/blackbeans/go-moa-client"
	"os"
	"os/signal"
)

type IGoMoaDemo interface {
	SetName(name string) error
	Ping() error
}

type GoMoaDemo struct {
}

func (self GoMoaDemo) SetName(name string) error {
	return nil
}

func (self GoMoaDemo) Ping() error {
	return nil
}

type GoMoaDemoProxy struct {
	SetName func(name string) error
	Ping    func() error
}

func main() {
	server := flag.Bool("server", true, "-server=true")
	flag.Parse()

	if *server {

		app := core.NewApplicationWithContext(context.TODO(), "./conf/moa.toml", func() []core.Service {
			return []core.Service{
				core.Service{
					ServiceUri: "/service/user-service",
					Instance:   GoMoaDemo{},
					Interface:  (*IGoMoaDemo)(nil)}}
		}, func(serviceUri, host string, moainfo core.MoaInfo) {

		})

		//设置
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Kill)
		//kill
		<-ch
		app.DestroyApplication()
	} else {
		startClient()
	}

}

func startClient() {
	consumer := client.NewMoaConsumer("./conf/moa.toml",
		[]client.Service{client.Service{
			ServiceUri: "/service/user-service",
			Interface:  &GoMoaDemoProxy{}}})

	for {
		s, _ := consumer.GetService("/service/user-service")
		h := s.(*GoMoaDemoProxy)
		err := h.SetName("a")
		if nil != err {

		}
	}
}
