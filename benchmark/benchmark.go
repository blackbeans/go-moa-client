package main

import (
	"flag"
	"github.com/blackbeans/go-moa-client"
	"github.com/blackbeans/go-moa/core"
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

		app := core.NewApplication("conf/moa.toml", func() []core.Service {
			return []core.Service{
				core.Service{
					ServiceUri: "/service/go-moa",
					Instance:   GoMoaDemo{},
					Interface:  (*IGoMoaDemo)(nil)}}
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
	consumer := go_moa_client.NewMoaConsumer("./conf/moa.toml",
		[]go_moa_client.Service{go_moa_client.Service{
			ServiceUri: "/service/go-moa",
			Interface:  &GoMoaDemoProxy{}}})

	for {
		s, _ := consumer.GetService("/service/go-moa")
		h := s.(*GoMoaDemoProxy)
		err := h.SetName("a")
		if nil != err {

		}
	}
}
