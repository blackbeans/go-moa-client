package main

import (
	"context"
	"flag"
	"github.com/blackbeans/go-moa-client/client"
	"github.com/blackbeans/go-moa/core"
	"github.com/blackbeans/log4go"
	"os"
	"os/signal"
)

type IGoMoaDemo interface {
	SetName(name string) (string, error)
	Ping() error
}

type GoMoaDemo struct {
}

func (self GoMoaDemo) SetName(name string) (string, error) {

	return name, nil
}

func (self GoMoaDemo) Ping() error {
	return nil
}

type GoMoaDemoProxy struct {
	SetName func(ctx context.Context, name string) (string, error)
	Ping    func() error
}

func main() {
	server := flag.Bool("server", true, "-server=true")
	flag.Parse()
	log4go.LoadConfiguration("conf/log.xml")
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
		select {}
	}

}

func startClient() {
	consumer := client.NewMoaConsumer("./conf/moa.toml",
		[]client.Service{client.Service{
			ServiceUri: "/service/go-moa",
			Interface:  &GoMoaDemoProxy{}}})

	s, _ := consumer.GetService("/service/go-moa")
	h := s.(*GoMoaDemoProxy)
	for {
		ctx := core.AttachMoaProperty(context.Background(), "Accept-Language", "zh-CN")
		_, err := h.SetName(ctx, "a")
		//fmt.Println(a)
		if nil != err {

		}
	}
}
