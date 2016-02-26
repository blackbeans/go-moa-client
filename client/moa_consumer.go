package client

import (
	"encoding/json"
	"git.wemomo.com/bibi/go-moa-client/option"
	"git.wemomo.com/bibi/go-moa/protocol"
	"git.wemomo.com/bibi/go-moa/proxy"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo/packet"
	"reflect"
)

type MoaConsumer struct {
	services      map[string]proxy.Service
	options       *option.ClientOption
	clientManager *MoaClientManager
}

func NewMoaConsumer(confPath string, ps []proxy.Service) *MoaConsumer {

	options, err := option.LoadConfiruation(confPath)
	if nil != err {
		panic(err)
	}

	services := make(map[string]proxy.Service, 2)
	consumer := &MoaConsumer{}
	uris := make([]string, 0, 2)
	for _, s := range ps {
		consumer.makeRpcFunc(s)
		services[s.ServiceUri] = s
		uris = append(uris, s.ServiceUri)
	}
	consumer.services = services
	consumer.options = options
	consumer.clientManager = NewMoaClientManager(options, uris)
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

		var outType reflect.Type
		//返回值
		if t.NumOut() >= 0 {
			outType = t.Out(0)
		}
		v := reflect.MakeFunc(t,
			func(s proxy.Service, methodName string, outType reflect.Type) func(in []reflect.Value) []reflect.Value {
				return func(in []reflect.Value) []reflect.Value {
					//包装RPC使用的参数
					defer func() {
						if err := recover(); nil != err {

						}
					}()
					return self.rpcInvoke(s, methodName, in, outType)
				}
			}(s, name, outType))
		method.Set(v)
	}
}

//真正发起RPC调用的逻辑
func (self MoaConsumer) rpcInvoke(s proxy.Service, method string,
	in []reflect.Value, outType reflect.Type) []reflect.Value {

	//1.选取服务地址
	c, err := self.clientManager.SelectClient(s.ServiceUri)
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|SelectClient|FAIL|%s|%s", err, s.ServiceUri)
		return []reflect.Value{reflect.ValueOf("hello")}
	}
	//2.组装请求协议
	cmd := protocol.CommandRequest{}
	cmd.ServiceUri = s.ServiceUri
	cmd.Params.Method = method
	args := make([]interface{}, 0, 3)
	for _, arg := range in {
		args = append(args, arg.Interface())
	}
	cmd.Params.Args = args
	//3.发送网络请求
	data, err := json.Marshal(cmd)
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|Marshal|FAIL|%s|%s|%s", err, cmd)
		return []reflect.Value{}
	}

	reqPacket := packet.NewPacket(0, data)
	//4.等待响应、超时、异常处理
	result, err := c.WriteAndGet(*reqPacket, self.options.ProcessTimeout)
	//5.返回调用结果
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|InvokeFail|%s|%s|%s", err, cmd)
		return []reflect.Value{}
	}

	out := reflect.New(outType).Interface()
	err = json.Unmarshal(result.([]byte), out)
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|Return Type Not Match|||%s|%s|%s", s.ServiceUri, method, string(result.([]byte)))
		return []reflect.Value{}
	}
	log.InfoLog("moa_client", "MoaConsumer|rpcInvoke|SUCC|%s|%s|%s", s.ServiceUri, method, string(result.([]byte)))
	return []reflect.Value{reflect.ValueOf(out)}
}
