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
		consumer.makeRpcFunc(s)
		services[s.ServiceUri] = s
	}
	consumer.services = services
	return consumer
}

func (self MoaConsumer) GetService(uri string) interface{} {
	return self.services[uri].Instance
}

func (self MoaConsumer) makeRpcFunc(s proxy.Service) {
	elem := reflect.ValueOf(s.Instance)
	obj := elem.Elem()
	numf := obj.NumField()
	htype := reflect.TypeOf(obj.Interface())
	for i := 0; i < numf; i++ {
		method := obj.Field(i)
		name := htype.Field(i).Name
		t := method.Type()
		v := reflect.MakeFunc(t, func(s proxy.Service, methodName string) func(in []reflect.Value) []reflect.Value {
			return func(in []reflect.Value) []reflect.Value {
				//包装RPC使用的参数
				defer func() {
					if err := recover(); nil != err {

					}
				}()
				return self.rpcInvoke(s, methodName, in)
			}
		}(s, name))
		method.Set(v)
	}
}

//真正发起RPC调用的逻辑
func (self MoaConsumer) rpcInvoke(s proxy.Service, method string,
	in []reflect.Value) []reflect.Value {

	//1.选取服务地址

	//2.组装请求协议

	//3.发送网络请求

	//4.等待响应、超时、异常处理

	//5.返回调用结果

	return []reflect.Value{reflect.ValueOf("hello")}
}
