package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/blackbeans/log4go"

	"github.com/blackbeans/go-moa"
	"github.com/blackbeans/turbo"
)

type MoaClientManager struct {
	clientsManager *turbo.ClientManager
	uri2Ips        map[string] /*uri*/ Strategy
	addrManager    *AddressManager
	op             core.Option
	snappy         bool
	lock           sync.RWMutex
	config         *turbo.TConfig
	ctx            context.Context
}

func NewMoaClientManager(ctx context.Context, option core.Option, uris []string) *MoaClientManager {

	cluster := option.Clusters[option.Client.RunMode]
	var reg core.IRegistry
	if strings.HasPrefix(cluster.Registry, core.SCHEMA_ZK) {
		reg = core.NewZkRegistry(strings.TrimPrefix(cluster.Registry, core.SCHEMA_ZK), uris, false)
	}

	reconnect := turbo.NewReconnectManager(true,
		10*time.Second, 10,
		func(ga *turbo.GroupAuth, remoteClient *turbo.TClient) (bool, error) {
			return true, nil
		})

	manager := &MoaClientManager{ctx: ctx}
	//参数
	manager.config =
		turbo.NewTConfig(
			"moa-client",
			cluster.MaxDispatcherSize,
			cluster.ReadBufferSize,
			cluster.ReadBufferSize,
			cluster.WriteChannelSize,
			cluster.ReadChannelSize,
			cluster.IdleTimeout,
			cluster.FutureSize)

	manager.op = option
	manager.clientsManager = turbo.NewClientManager(reconnect)
	manager.uri2Ips = make(map[string]Strategy, 2)
	if strings.ToLower(option.Client.Compress) == "snappy" {
		manager.snappy = true
	}
	addrManager := NewAddressManager(reg, uris, manager.OnAddressChange)
	manager.addrManager = addrManager

	go manager.CheckAlive()
	return manager
}

func (self *MoaClientManager) CheckAlive() {
	t := time.Tick(5 * time.Second)
	for {
		<-t
		for hp, c := range self.clientsManager.ClientsClone() {
			ip := hp
			server := c
			//如果当前client是空闲的则需要发送Ping-pong
			if c.Idle() {
				go func() {
					pipo := core.PiPo{Timestamp: time.Now().Unix()}
					p := turbo.NewPacket(core.PING, nil)
					p.PayLoad = pipo
					err := server.Ping(p, self.op.Clusters[self.op.Client.RunMode].ProcessTimeout)
					if nil != err {
						log4go.WarnLog("config_center", "CheckAlive|FAIL|%s|%v", ip, err)
					} else {
						log4go.WarnLog("config_center", "CheckAlive|SUCC|%s...", ip)
					}
				}()
			}
		}

	}
}

func (self *MoaClientManager) OnAddressChange(uri string, hosts []string) {
	log4go.WarnLog("config_center", "OnAddressChange|%s|%s", uri, hosts)
	//新增地址
	addHostport := make([]string, 0, 2)

	self.lock.Lock()
	//寻找新增连接
	for _, ip := range hosts {
		exist := self.clientsManager.FindTClient(ip)
		if nil == exist {
			addHostport = append(addHostport, ip)
		}
	}

	for _, hp := range addHostport {

		connection, err := net.DialTimeout("tcp", hp, self.op.Clusters[self.op.Client.RunMode].ProcessTimeout*5)
		if nil != err {
			log4go.ErrorLog("config_center", "MoaClientManager|Create Client|FAIL|%s|%v", hp, err)
			continue
		}
		conn := connection.(*net.TCPConn)

		c := turbo.NewTClient(self.ctx, conn, func() turbo.ICodec {
			return core.BinaryCodec{
				MaxFrameLength: turbo.MAX_PACKET_BYTES,
				SnappyCompress: self.snappy}
		}, self.dis, self.config)
		c.Start()
		log4go.InfoLog("config_center", "MoaClientManager|Create Client|SUCC|%s", hp)
		self.clientsManager.Auth(turbo.NewGroupAuth(hp, ""), c)
	}

	if self.op.Client.SelectorStrategy == core.STRATEGY_KETAMA {
		self.uri2Ips[uri] = NewKetamaStrategy(hosts)
	} else if self.op.Client.SelectorStrategy == core.STRATEGY_RANDOM {
		self.uri2Ips[uri] = NewRandomStrategy(hosts)
	} else {
		self.uri2Ips[uri] = NewRandomStrategy(hosts)
	}

	log4go.InfoLog("config_center", "MoaClientManager|Store Uri Pool|SUCC|%s|%v", uri, hosts)
	//清理掉不再使用client
	usingIps := make(map[string]bool, 5)
	for _, v := range self.uri2Ips {
		v.Iterator(func(i int, ip string) {
			usingIps[ip] = true
		})
	}

	for ip := range self.clientsManager.ClientsClone() {
		_, ok := usingIps[ip]
		if !ok {
			//不再使用了移除
			self.clientsManager.DeleteClients(ip)
			log4go.InfoLog("config_center", "MoaClientManager|RemoveUnUse Client|SUCC|%s", ip)
		}
	}
	self.lock.Unlock()

	log4go.InfoLog("config_center", "MoaClientManager|OnAddressChange|SUCC|%s|%v", uri, hosts)
}

//根据Uri获取连接
func (self *MoaClientManager) SelectClient(uri string, key string) (*turbo.TClient, error) {

	self.lock.RLock()
	defer self.lock.RUnlock()

	strategy, ok := self.uri2Ips[uri]
	if ok {
		ip := strategy.Select(key)
		if len(ip) > 0 {
			p := self.clientsManager.FindTClient(ip)
			if nil != p {
				return p, nil
			}
		}
	}
	return nil, errors.New(fmt.Sprintf("NO CLIENT FOR %s", uri))

}

func (self *MoaClientManager) Destroy() {
	self.clientsManager.Shutdown()
}

//设置
func (self *MoaClientManager) dis(ctx *turbo.TContext) error {
	p := ctx.Message
	if p.Header.CmdType == core.PONG {
		pipo := p.PayLoad.(core.PiPo)
		ctx.Client.Attach(p.Header.Opaque, pipo.Timestamp)
	} else {
		ctx.Client.Attach(p.Header.Opaque, p.PayLoad)
	}
	return nil
}
