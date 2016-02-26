package client

import (
	"git.wemomo.com/bibi/go-moa/proxy"
	// "runtime"
	// "fmt"
	"testing"
)

type Hello struct {
	GetService func(serviceUri, proto string) string
}

func TestMakeRpcFunc(t *testing.T) {

	consumer := NewMoaConsumer("../conf/moa_client.toml",
		[]proxy.Service{proxy.Service{
			ServiceUri: "/service/lookup",
			Instance:   &Hello{}}})
	h := consumer.GetService("/service/lookup").(*Hello)
	a := h.GetService("a", "b")
	t.Log(a)
	if a != "hello" {
		t.Fail()
	}

}
