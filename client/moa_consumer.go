package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/blackbeans/go-moa-client/option"
	"github.com/blackbeans/go-moa/protocol"
	"github.com/blackbeans/go-moa/proxy"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo/packet"
	"reflect"
	"strings"
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
		uri := s.ServiceUri + options.ServiceUriSuffix
		s.ServiceUri = uri
		services[uri] = s
		uris = append(uris, uri)
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

func (self MoaConsumer) Destory() {
	self.clientManager.Destory()
}

func (self MoaConsumer) GetService(uri string) interface{} {
	return self.services[uri].Interface
}

func (self MoaConsumer) makeRpcFunc(s proxy.Service) {
	elem := reflect.ValueOf(s.Interface)
	obj := elem.Elem()
	numf := obj.NumField()
	htype := reflect.TypeOf(obj.Interface())
	for i := 0; i < numf; i++ {
		method := obj.Field(i)
		//fuck 统一约定方法首字母小写
		name := strings.ToLower(string(htype.Field(i).Name[0])) +
			htype.Field(i).Name[1:]
		t := method.Type()
		outType := make([]reflect.Type, 0, 2)
		//返回值必须大于等于1个并且小于2，并且其中一个必须为error类型
		if t.NumOut() >= 1 && t.NumOut() <= 2 {
			for idx := 0; idx < t.NumOut(); idx++ {
				outType = append(outType, t.Out(idx))
			}
			if !outType[len(outType)-1].Implements(errorType) {
				panic(errors.New(
					fmt.Sprintf("%s Method  %s Last Return Type Must Be An Error! [%s]",
						s.ServiceUri, name, outType[len(outType)-1].String())))
			}
		} else {
			panic(errors.New(
				fmt.Sprintf("%s Method  %s Last Return Type Must Be More Than An Error!", s.ServiceUri, name)))
		}

		f := func(s proxy.Service, methodName string,
			outType []reflect.Type) func(in []reflect.Value) []reflect.Value {
			return func(in []reflect.Value) []reflect.Value {
				vals := self.rpcInvoke(s, methodName, in, outType)
				return vals
			}
		}(s, name, outType)
		v := reflect.MakeFunc(t, f)
		method.Set(v)
	}
}

var errorType = reflect.TypeOf(make([]error, 1)).Elem()

//真正发起RPC调用的逻辑
func (self MoaConsumer) rpcInvoke(s proxy.Service, method string,
	in []reflect.Value, outType []reflect.Type) []reflect.Value {

	errFunc := func(err *error) []reflect.Value {
		retVal := make([]reflect.Value, 0, len(outType))
		for _, t := range outType {
			if t.Implements(errorType) {
				retVal = append(retVal, reflect.ValueOf(err).Elem())
			} else {
				retVal = append(retVal, reflect.New(t).Elem())
			}
		}

		return retVal
	}

	//1.选取服务地址
	c, err := self.clientManager.SelectClient(s.ServiceUri)
	//fmt.Printf("MoaConsumer|rpcInvoke|SelectClient|FAIL|%s|%s\n", err, s.ServiceUri)
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|SelectClient|FAIL|%s|%s",
			err, s.ServiceUri)
		return errFunc(&err)

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
	//fmt.Printf("MoaConsumer|rpcInvoke|Marshal|FAIL|%s|%s|%s\n", err, cmd)
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|Marshal|FAIL|%s|%s", err, cmd)
		return errFunc(&err)
	}

	reqPacket := packet.NewRespPacket(0, 0, data)
	//4.等待响应、超时、异常处理
	result, err := c.WriteAndGet(*reqPacket, self.options.ProcessTimeout)
	//5.返回调用结果
	//fmt.Printf("MoaConsumer|rpcInvoke|InvokeFail|%s|%s|%s\n", err, cmd)
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|InvokeFail|%s|%s", err, cmd)
		return errFunc(&err)
	}

	var resp protocol.MoaRespPacket
	err = json.Unmarshal(result.([]byte), &resp)
	//fmt.Printf("MoaConsumer|rpcInvoke|Return Type Not Match|%s|%s|%s\n", s.ServiceUri, method, string(result.([]byte)))
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|Return Type Not Match|%s|%s|%s", s.ServiceUri, method, string(result.([]byte)))
		return errFunc(&err)
	}

	//获取非Error的返回类型
	var resultType reflect.Type
	for _, t := range outType {
		if !t.Implements(errorType) {
			resultType = t
		}
	}

	//执行成功
	if resp.ErrCode == protocol.CODE_SERVER_SUCC {
		if nil != resultType {
			vl := reflect.ValueOf(resp.Result)
			//类型相等应该就是原则类型了吧、不是数组并且两个类型一样
			if resultType == vl.Type() {
				return []reflect.Value{vl, reflect.Zero(errorType)}
			} else if vl.Kind() == reflect.Map ||
				vl.Kind() == reflect.Slice {
				//可能是对象类型则需要序列化为该对象
				data, err := json.Marshal(resp.Result)
				if nil != err {
					log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|Marshal|FAIL|%s|%s|%s", s.ServiceUri, method, resp.Result)
					return errFunc(&err)
				} else {
					inst := reflect.New(resultType)

					if len(data) > 0 {
						uerr := json.Unmarshal(data, inst.Interface())
						if nil != uerr {
							return errFunc(&uerr)
						}
					}
					return []reflect.Value{inst.Elem(), reflect.Zero(errorType)}
				}
			} else {

				log.ErrorLog("moa_client",
					"MoaConsumer|rpcInvoke|UnSupport Return Type|%s|%s|%s|%s",
					s.ServiceUri, method, resp.Result, resultType.String())
				err = errors.New(fmt.Sprintf("UnSupport Return Type|%s|%s|%s|%s",
					resp.Result, resultType.String()))
				return errFunc(&err)
			}
		} else {
			//只有error的情况,没有错误返回成功
			return []reflect.Value{reflect.Zero(errorType)}
		}
	} else {
		//invoke Fail
		log.ErrorLog("moa_client",
			"MoaConsumer|rpcInvoke|RPC FAIL|%s|%s|%s",
			s.ServiceUri, method, resp)
		err = errors.New(resp.Message)
		return errFunc(&err)
	}
}
