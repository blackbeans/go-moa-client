package client

import (
	"git.wemomo.com/bibi/go-moa/proxy"
	"reflect"
)

type MoaConsumer struct {
	services map[string]proxy.Service
	// handler  *proxy.InvocationHandler
}

func NewMoaConsumer(ps []proxy.Service) *MoaConsumer {
	services := make(map[string]proxy.Service, 2)
	consumer := &MoaConsumer{}
	for _, s := range ps {
		consumer.makeRpcFunc(s.Instance)
		services[s.ServiceUri] = s
	}
	consumer.services = services
	return consumer
}

func (self MoaConsumer) GetService(uri string) interface{} {
	return self.services[uri].Instance
}

func (self MoaConsumer) makeRpcFunc(h interface{}) {
	elem := reflect.ValueOf(h)
	obj := elem.Elem()
	numf := obj.NumField()
	htype := reflect.TypeOf(obj.Interface())
	for i := 0; i < numf; i++ {
		method := obj.Field(i)
		name := htype.Field(i).Name
		t := method.Type()
		v := reflect.MakeFunc(t, func(name string) func(in []reflect.Value) []reflect.Value {
			return func(in []reflect.Value) []reflect.Value {
				//包装RPC使用的参数

				return []reflect.Value{reflect.ValueOf("hello")}
			}
		}(name))
		method.Set(v)
	}
}
