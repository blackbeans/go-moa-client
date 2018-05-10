package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/blackbeans/go-moa/core"
	"github.com/blackbeans/go-moa/proto"

	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo/packet"
)

type Service struct {
	ServiceUri string                       //serviceUr对应的服务名称
	GroupIds   []string                     //该服务的分组
	group2Uri  map[string] /*group*/ string //compose uri
	Interface  interface{}
}

type MoaConsumer struct {
	services      map[string]core.Service
	options       core.Option
	clientManager *MoaClientManager
	buffPool      *sync.Pool
}

func NewMoaConsumer(confPath string, ps []Service) *MoaConsumer {

	options, err := core.LoadConfiruation(confPath)
	if nil != err {
		panic(err)
	}
	options = core.InitClientOption(options)

	services := make(map[string]core.Service, 2)
	consumer := &MoaConsumer{}
	globalUnique := make(map[string]*interface{}, 10)
	for _, s := range ps {
		s.GroupIds = append(s.GroupIds, "*")
		for _, g := range s.GroupIds {
			//全部转为指针类型的
			instType := reflect.TypeOf(s.Interface)
			if instType.Kind() == reflect.Ptr {
				instType = instType.Elem()
			}
			clone := reflect.New(instType).Interface()
			uri := BuildServiceUri(s.ServiceUri, g)
			services[uri] = core.Service{
				ServiceUri: s.ServiceUri,
				GroupId:    g,
				Interface:  clone}
			globalUnique[uri] = nil
		}
	}

	//去重
	uris := make([]string, 0, len(globalUnique))
	for uri := range globalUnique {
		uris = append(uris, uri)
	}
	consumer.services = services
	consumer.options = options

	pool := &sync.Pool{}
	pool.New = func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	}
	consumer.buffPool = pool
	//添加代理
	for _, s := range services {
		consumer.makeRpcFunc(s)
	}
	consumer.clientManager = NewMoaClientManager(options, uris)
	return consumer
}

func BuildServiceUri(serviceUri string, groupid string) string {
	if len(groupid) > 0 && "*" != groupid {
		return fmt.Sprintf("%s#%s", serviceUri, groupid)
	} else {
		return serviceUri
	}
}

func splitServiceUri(serviceUri string) (uri, groupId string) {
	if strings.IndexAny(serviceUri, "#") >= 0 {
		split := strings.SplitN(serviceUri, "#", 2)
		return split[0], split[1]
	} else {
		return serviceUri, "*"
	}
}

func (self *MoaConsumer) Destroy() {
	self.clientManager.Destroy()
}

var ERR_NO_SERVICE = errors.New("No Exist Service ")

func (self *MoaConsumer) GetService(uri string) (interface{}, error) {

	proxy, ok := self.services[uri]
	if ok {
		return proxy.Interface, nil
	} else {
		return nil, ERR_NO_SERVICE
	}
}

func (self *MoaConsumer) GetServiceWithGroupid(uri, groupid string) (interface{}, error) {
	proxy, ok := self.services[BuildServiceUri(uri, groupid)]
	if ok {
		return proxy.Interface, nil
	} else {
		return nil, ERR_NO_SERVICE
	}
}

func (self *MoaConsumer) makeRpcFunc(s core.Service) {

	elem := reflect.ValueOf(s.Interface)
	obj := elem.Elem()
	numf := obj.NumField()
	htype := reflect.TypeOf(obj.Interface())
	for i := 0; i < numf; i++ {
		method := obj.Field(i)

		//处理嵌套继承
		if method.Kind() == reflect.Struct{
			structType := method.Type()
			for j:=0  ;j< method.NumField();j++{
				function  := method.Field(j)
				//如果是func类型那么就反射
				if function.Kind() == reflect.Func{
					self.proxyMethod(s,structType,j,function)

				}else{
					//如果是别的类型，这里就说不过去了。潜逃了好几层
					panic(fmt.Errorf("ServiceProxy :[%s->%s]Too Deep  Nesting !",s.ServiceUri,obj.String()))
				}
			}
		}else {
			self.proxyMethod(s,htype,i,method)
		}
	}
	log.InfoLog("moa_client", "MoaConsumer|Proxy|SUCC|%s->%s", s.ServiceUri, s.GroupId)
}

//动态代理调用方法
func (self *MoaConsumer)proxyMethod(s core.Service,htype reflect.Type,i int,method reflect.Value){
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
			panic(fmt.Errorf("%s Method  %s Last Return Type Must Be An Error! [%s]",
				s.ServiceUri, name, outType[len(outType)-1].String()))
		}
	} else {
		panic(fmt.Errorf("%s Method  %s Last Return Type Must Be More Than An Error!", s.ServiceUri, name))
	}

	f := func(s core.Service, methodName string,
		outType []reflect.Type) func(in []reflect.Value) []reflect.Value {
		return func(in []reflect.Value) []reflect.Value {
			vals := self.rpcInvoke(s, methodName, in, outType)
			return vals
		}
	}(s, name, outType)
	v := reflect.MakeFunc(t, f)
	method.Set(v)
}

var errorType = reflect.TypeOf(make([]error, 1)).Elem()

//真正发起RPC调用的逻辑
func (self *MoaConsumer) rpcInvoke(s core.Service, method string,
	in []reflect.Value, outType []reflect.Type) []reflect.Value {

	errFunc := func(err error) []reflect.Value {
		retVal := make([]reflect.Value, 0, len(outType))
		for _, t := range outType {
			if t.Implements(errorType) {
				retVal = append(retVal, reflect.ValueOf(&err).Elem())
			} else {
				retVal = append(retVal, reflect.New(t).Elem())
			}
		}

		return retVal
	}

	//1.组装请求协议
	cmd := proto.MoaReqPacket{}
	cmd.ServiceUri = s.ServiceUri
	cmd.Params.Method = method
	args := make([]interface{}, 0, 3)

	buff := self.buffPool.Get().(*bytes.Buffer)
	defer func() {
		buff.Reset()
		self.buffPool.Put(buff)
	}()

	for i, arg := range in {
		if i <= 0 {
			buff.WriteString(arg.String())
		}
		args = append(args, arg.Interface())
	}
	cmd.Params.Args = args

	//2.选取服务地址
	serviceUri := BuildServiceUri(s.ServiceUri, s.GroupId)
	c, err := self.clientManager.SelectClient(serviceUri, buff.String())
	if nil != err {
		log.ErrorLog("moa_client", "MoaConsumer|rpcInvoke|SelectClient|FAIL|%s|%s",
			err, serviceUri)
		return errFunc(err)

	}

	//4.等待响应、超时、异常处理
	req := packet.NewPacket(proto.REQ, nil)
	req.PayLoad = cmd
	response, err := c.WriteAndGet(*req,
		self.options.Clusters[self.options.Client.RunMode].ProcessTimeout)

	if nil != err {
		//response error and close this connection
		log.ErrorLog("moa_client", "MoaConsumer|InvokeFail|%s|%s", err, cmd)
		return errFunc(err)
	}

	resp := response.(proto.MoaRawRespPacket)
	//获取非Error的返回类型
	var resultType reflect.Type
	for _, t := range outType {
		if !t.Implements(errorType) {
			resultType = t
		}
	}

	//执行成功
	if resp.ErrCode == proto.CODE_SERVER_SUCC {
		var respErr reflect.Value
		if len(resp.Message) > 0 {
			//好恶心的写法
			createErr := errors.New(resp.Message)
			respErr = reflect.ValueOf(&createErr).Elem()
		} else {
			respErr = reflect.Zero(errorType)
		}

		if nil != resultType {
			//可能是对象类型则需要序列化为该对象
			inst := reflect.New(resultType)
			uerr := json.Unmarshal(resp.Result, inst.Interface())
			if nil != uerr {
				return errFunc(uerr)
			}
			return []reflect.Value{inst.Elem(), respErr}

		} else {
			//只有error的情况,没有错误返回成功
			return []reflect.Value{respErr}
		}
	} else {
		//invoke Fail
		log.ErrorLog("moa_client",
			"MoaConsumer|RPC FAIL|%s|%s|%v",
			s.ServiceUri, method, resp)
		err = errors.New(resp.Message)
		return errFunc(err)
	}
}
