package client

import (
	"errors"
	"fmt"
	"github.com/blackbeans/go-moa-client/option"
	"github.com/blackbeans/go-moa/lb"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
	"math"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

type MoaClientManager struct {
	clientManager *client.ClientManager
	clientPool    map[string]chan bool //redis连接不能被多线程复用，必须占位
	addrManager   *AddressManager
	rc            *turbo.RemotingConfig
	op            *option.ClientOption
	lock          sync.RWMutex
}

const (
	REGISTRY_TYPE_MOMOKEEPER = "momokeeper"
	REGISTRY_TYPE_ZOOKEEPER  = "zookeeper"
)

func NewMoaClientManager(op *option.ClientOption, uris []string) *MoaClientManager {
	var reg lb.IRegistry
	if op.RegistryType == REGISTRY_TYPE_MOMOKEEPER {
		split := strings.Split(op.RegistryHosts, ",")
		if len(split) > 1 {
			reg = lb.NewMomokeeper(split[0], split[1])
		} else {
			reg = lb.NewMomokeeper(split[0], split[0])
		}

	} else if op.RegistryType == REGISTRY_TYPE_ZOOKEEPER {
		reg = lb.NewZookeeper(op.RegistryHosts, uris)
	}

	//网络参数
	rc := turbo.NewRemotingConfig(
		op.AppName,
		op.PoolSizePerHost, 16*1024,
		16*1024, 10000, 10000,
		1*time.Minute, 10)

	manager := &MoaClientManager{}
	manager.clientPool = make(map[string]chan bool, 100)
	//创建连接重连握手回调
	reconn := client.NewReconnectManager(true, 5*time.Second, 10,
		func(ga *client.GroupAuth, remoteClient *client.RemotingClient) (bool, error) {
			//moa中没有握手返回成功
			return true, nil
		})
	manager.op = op
	manager.rc = rc
	manager.clientManager = client.NewClientManager(reconn)

	addrManager := NewAddressManager(reg, uris, manager.OnAddressChange)
	manager.addrManager = addrManager

	//开启服务PING PONG
	go func() {
		for {
			time.Sleep(op.ProcessTimeout * 2)
			wg := sync.WaitGroup{}
			clone := manager.clientManager.ClientsClone()
			wg.Add(len(clone))
			for _, c := range clone {
				go func() {
					defer wg.Done()
					manager.ping(c)
				}()
			}
			//等待本次的所有的PING—PONG结束
			wg.Wait()
		}
	}()
	return manager
}

func (self MoaClientManager) ping(c *client.RemotingClient) bool {
	succ := false
	//发起PING请求
	for i := 0; i < 2; i++ {
		if c.Idle() && !c.IsClosed() {
			func() {
				self.lock.RLock()
				defer self.lock.RUnlock()
				select {
				case <-self.clientPool[c.LocalAddr()]:
				case <-time.After(self.op.ProcessTimeout * 2):
					//如果超时还没得到连接可用则放弃本次PING，因为连接正忙
					succ = true
					return
				}
				//发送PING协议
				heartbeat := packet.NewRespPacket(0, CMD_PING, PING)
				err := c.Ping(heartbeat, self.op.ProcessTimeout)
				if nil != err {
					//需要关闭连接，然后重连任务会启动重连
					if i == 1 {
						c.Shutdown()
						log.WarnLog("address_manager",
							"MoaClientManager|checkConnStatus|Ping|FAIL|%s|%s", c.LocalAddr(), err)
						succ = false
						return
					} else {
						//第一次超时等待第二次PING
						delay := time.Duration(int64(math.Pow(2, float64(i+1))) * int64(self.op.ProcessTimeout))
						time.Sleep(delay)
					}
				} else {
					succ = true
					log.InfoLog("address_manager",
						"MoaClientManager|checkConnStatus|Ping|SUCC|%s...", c.LocalAddr())
					return
				}
			}()
		}
	}

	return succ

}

func (self MoaClientManager) OnAddressChange(uri string, hosts []string) {
	removeHostport := make([]string, 0, 2)
	//创建连接
	remoteClients := self.clientManager.FindRemoteClients([]string{uri},
		func(groupId string, rc *client.RemotingClient) bool {
			exist := false
			hostport := rc.RemoteAddr()
			addr, _ := net.ResolveTCPAddr("tcp4", hostport)
			if addr.IP.To4().IsLoopback() {
				hostport = fmt.Sprintf("0.0.0.0:%d", addr.Port)
			}
			for _, h := range hosts {
				if strings.HasPrefix(h, hostport) {
					exist = true
					break
				}
			}
			//如果不在对应列表中则是需要删除的
			if !exist {
				removeHostport = append(removeHostport, hostport)
			}
			//不存在直接过滤
			return !exist
		})

	//新增地址
	addHostport := make([]string, 0, 2)
	clients, ok := remoteClients[uri]
	//如果不存在则新增
	if !ok || len(clients) <= 0 {
		//创建连接
		addHostport = hosts
	} else {
		//如果存在则不新增

		for _, h := range hosts {
			exist := false
			for _, c := range clients {
				if strings.HasPrefix(h, c.RemoteAddr()) {
					exist = true
					break
				}

			}
			//有新增的地址
			if !exist {
				addHostport = append(addHostport, h)
			}
		}
	}
	//新增创建
	for _, h := range addHostport {
		split := strings.Split(h, "?")
		conn, err := dial(split[0])
		if nil == err {

			//需要开发对应的codec
			cf := func() codec.ICodec {
				decoder := MoaClientCodeC{}
				decoder.MaxFrameLength = 32 * 1024
				return decoder
			}

			//创建一下连接数
			for i := 0; i < self.op.PoolSizePerHost; i++ {
				//创建连接
				remoteClient := client.NewRemotingClient(conn, cf, self.readDispatcher, self.rc)
				remoteClient.Start()
				auth := client.NewGroupAuth(uri, self.op.AppSecretKey)
				succ := self.clientManager.Auth(auth, remoteClient)
				if succ {
					func() {
						self.lock.Lock()
						defer self.lock.Unlock()
						ch := make(chan bool, 1)
						ch <- true
						self.clientPool[remoteClient.LocalAddr()] = ch

					}()
				}
				log.InfoLog("address_manager", "MoaClientManager|OnAddressChange|Auth|SUCC|%s[%d]|%s|%v", uri, i, h, succ)
			}
		} else {
			log.WarnLog("address_manager", "MoaClientManager|OnAddressChange|Auth|FAIL|%s|%s|%s", err, uri, h)
		}

	}
	if len(removeHostport) > 0 {
		//删除需要删除的客户端
		self.clientManager.DeleteClients(removeHostport...)
		log.WarnLog("address_manager", "MoaClientManager|OnAddressChange|RemoveClients|%s", removeHostport)
	}
	func() {

		self.lock.Lock()
		defer self.lock.Unlock()
		for _, h := range removeHostport {
			delete(self.clientPool, h)
		}
	}()
}

//需要开发对应的分包
func (self MoaClientManager) readDispatcher(remoteClient *client.RemotingClient, p *packet.Packet) {
	//直接写过去[]byte结构
	switch p.Header.CmdType {
	case CMD_GET:
		remoteClient.Attach(p.Header.Opaque, p.Data)
	case CMD_PING:
		//如果是PONG数据则attach当前时间戳
		remoteClient.Attach(p.Header.Opaque, time.Now().Unix())
	}

	self.lock.RLock()
	defer self.lock.RUnlock()
	self.clientPool[remoteClient.LocalAddr()] <- true
}

//根据Uri获取连接
func (self MoaClientManager) SelectClient(uri string) (*client.RemotingClient, error) {
	remoteClients := self.clientManager.FindRemoteClients([]string{uri},
		func(groupId string, rc *client.RemotingClient) bool {
			return false
		})

	clients, ok := remoteClients[uri]
	if !ok || len(clients) <= 0 {
		return nil, errors.New("NO Client for [" + uri + "]")
	} else {
		c := clients[rand.Intn(len(clients))]
		self.lock.RLock()
		defer self.lock.RUnlock()
		select {
		case <-self.clientPool[c.LocalAddr()]:
			return c, nil
		case <-time.After(self.op.ProcessTimeout):
			return nil, errors.New(fmt.Sprintf("SelectClient Timeout %s", uri))
		}

	}

}

func (self MoaClientManager) Destory() {
	self.clientManager.Shutdown()
}

//创建物理连接
func dial(hostport string) (*net.TCPConn, error) {
	//连接
	remoteAddr, err_r := net.ResolveTCPAddr("tcp4", hostport)
	if nil != err_r {
		log.ErrorLog("address_manager", "MoaClientManager|DIAL|RESOLVE ADDR |FAIL|remote:%s\n", err_r)
		return nil, err_r
	}
	conn, err := net.DialTCP("tcp4", nil, remoteAddr)
	if nil != err {
		log.ErrorLog("address_manager", "MoaClientManager|DIAL|%s|FAIL|%s\n", hostport, err)
		return nil, err
	}

	return conn, nil
}
