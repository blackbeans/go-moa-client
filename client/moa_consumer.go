package client

import (
	"encoding/json"
	// "fmt"
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
		services[s.ServiceUri] = s
		uris = append(uris, s.ServiceUri)
	}
	consumer.services = services
	consumer.options = options
	consumer.clientManager = NewMoaClientManager(options, uris)
	//添加代理
	for _, s := range ps {
		consumer.makeRpcFunc(s)
	}
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

		f := func(s proxy.Service, methodName string,
			outType reflect.Type) func(in []reflect.Value) []reflect.Value {
			return func(in []reflect.Value) []reflect.Value {
				vals := self.rpcInvoke(s, methodName, in, outType)
				return vals
			}
		}(s, name, outType)
		v := reflect.MakeFunc(t, f)
		method.Set(v)
	}
}

//真正发起RPC调用的逻辑
func (self MoaConsumer) rpcInvoke(s proxy.Service, method string,
	in []reflect.Value, outType reflect.Type) []reflect.Value {

	//包装RPC使用的参数
	defer func() {
		if err := recover(); nil != err {

		}
	}()

	//1.选取服务地址
	c, err := self.clientManager.SelectClient(s.ServiceUri)
	// fmt.Printf("MoaConsumer|rpcInvoke|SelectClient|FAIL|%s|%s\n",err, s.ServiceUri)
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|SelectClient|FAIL|%s|%s",
			err, s.ServiceUri)
		return []reflect.Value{reflect.New(outType).Elem()}
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
	// fmt.Printf("MoaConsumer|rpcInvoke|Marshal|FAIL|%s|%s|%s\n", err, cmd)
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|Marshal|FAIL|%s|%s", err, cmd)
		return []reflect.Value{reflect.New(outType).Elem()}
	}

	reqPacket := packet.NewRespPacket(0, 0, data)
	//4.等待响应、超时、异常处理
	result, err := c.WriteAndGet(*reqPacket, self.options.ProcessTimeout)
	//5.返回调用结果
	//fmt.Printf("MoaConsumer|rpcInvoke|InvokeFail|%s|%s|%s\n", err, cmd)
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|InvokeFail|%s|%s", err, cmd)
		return []reflect.Value{reflect.New(outType).Elem()}
	}

	var resp protocol.MoaRespPacket
	err = json.Unmarshal(result.([]byte), &resp)
	//fmt.Printf("MoaConsumer|rpcInvoke|Return Type Not Match|%s|%s|%s\n", s.ServiceUri, method, string(result.([]byte)))
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|Return Type Not Match|%s|%s|%s", s.ServiceUri, method, string(result.([]byte)))
		return []reflect.Value{reflect.New(outType).Elem()}
	}

	//执行成功
	if resp.ErrCode == protocol.CODE_SERVER_SUCC {
		//log.InfoLog("moa_client", "MoaConsumer|rpcInvoke|SUCC|%s|%s|%s", s.ServiceUri, method, string(result.([]byte)))
		if nil != outType {
			vl := reflect.ValueOf(resp.Result)
			//类型相等应该就是原则类型了吧、不是数组并且两个类型一样
			if outType == vl.Type() {
				return []reflect.Value{vl}
			} else if vl.Kind() == reflect.Map ||
				vl.Kind() == reflect.Slice {
				//可能是对象类型则需要序列化为该对象
				data, err := json.Marshal(resp.Result)
				if nil != err {
					log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|Marshal|FAIL|%s|%s|%s", s.ServiceUri, method, resp.Result)
					panic(err)
				} else {
					inst := reflect.New(outType)
					uerr := json.Unmarshal(data, inst.Interface())
					if nil != uerr {
						panic(uerr)
					}
					return []reflect.Value{inst.Elem()}
				}
			} else {

				log.ErrorLog("moa_client",
					"MoaConsumer|rpcInvoke|UnSupport Return Type|%s|%s|%s|%s",
					s.ServiceUri, method, resp.Result, outType.String())
				// return []reflect.Value{reflect.New(outType).Elem()}
			}
		} else {
			return []reflect.Value{reflect.New(outType)}
		}
	} else {
		//invoke Fail
		log.ErrorLog("moa_client",
			"MoaConsumer|rpcInvoke|RPC FAIL|%s|%s|%s",
			s.ServiceUri, method, resp)
		return []reflect.Value{reflect.New(outType).Elem()}
	}
	return []reflect.Value{reflect.New(outType).Elem()}
}
