package client

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"

	"github.com/blackbeans/go-moa/core"
	"github.com/blackbeans/go-moa/lb"
	"github.com/blackbeans/go-moa/proto"
	log "github.com/blackbeans/log4go"
	tclient "github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
)

type MoaClientManager struct {
	clientsManager *tclient.ClientManager
	uri2Ips        map[string] /*uri*/ core.Strategy
	addrManager    *AddressManager
	op             core.Option
	snappy         bool
	lock           sync.RWMutex
	remoteConfig   *turbo.RemotingConfig
}

func NewMoaClientManager(option core.Option, uris []string) *MoaClientManager {

	cluster := option.Clusters[option.Client.RunMode]
	var reg lb.IRegistry
	if strings.HasPrefix(cluster.Registry, core.SCHEMA_ZK) {
		reg = lb.NewZkRegistry(strings.TrimPrefix(cluster.Registry, core.SCHEMA_ZK), uris, false)
	}

	reconnect := tclient.NewReconnectManager(true,
		10*time.Second, 10,
		func(ga *tclient.GroupAuth, remoteClient *tclient.RemotingClient) (bool, error) {
			return true, nil
		})

	manager := &MoaClientManager{}
	//参数
	manager.remoteConfig = turbo.NewRemotingConfig(
		"moa-client",
		cluster.MaxDispatcherSize,
		cluster.ReadBufferSize,
		cluster.ReadBufferSize,
		cluster.WriteChannelSize,
		cluster.ReadChannelSize,
		cluster.IdleTimeout,
		50*10000)

	manager.op = option
	manager.clientsManager = tclient.NewClientManager(reconnect)
	manager.uri2Ips = make(map[string]core.Strategy, 2)
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
					pipo := proto.PiPo{Timestamp: time.Now().Unix()}
					p := packet.NewPacket(proto.PING, nil)
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
		exist := self.clientsManager.FindRemoteClient(ip)
		if nil == exist {
			addHostport = append(addHostport, ip)
		}
	}

	for _, hp := range addHostport {

		connection, err := net.DialTimeout("tcp", hp, self.op.Clusters[self.op.Client.RunMode].ProcessTimeout*5)
		if nil != err {
			log.ErrorLog("config_center", "MoaClientManager|Create Client|FAIL|%s|%v", hp, err)
			continue
		}
		conn := connection.(*net.TCPConn)

		c := tclient.NewRemotingClient(conn, func() codec.ICodec {
			return proto.BinaryCodec{
				MaxFrameLength: packet.MAX_PACKET_BYTES,
				SnappyCompress: self.snappy}
		}, self.packetDis, self.remoteConfig)
		c.Start()
		log.InfoLog("config_center", "MoaClientManager|Create Client|SUCC|%s", hp)
		self.clientsManager.Auth(tclient.NewGroupAuth(hp, ""), c)
	}

	if self.op.Client.SelectorStrategy == core.STRATEGY_KETAMA {
		self.uri2Ips[uri] = core.NewKetamaStrategy(hosts)
	} else if self.op.Client.SelectorStrategy == core.STRATEGY_RANDOM {
		self.uri2Ips[uri] = core.NewRandomStrategy(hosts)
	} else {
		self.uri2Ips[uri] = core.NewRandomStrategy(hosts)
	}

	log.InfoLog("config_center", "MoaClientManager|Store Uri Pool|SUCC|%s|%v", uri, hosts)
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
			log.InfoLog("config_center", "MoaClientManager|RemoveUnUse Client|SUCC|%s", ip)
		}
	}
	self.lock.Unlock()

	log.InfoLog("config_center", "MoaClientManager|OnAddressChange|SUCC|%s|%v", uri, hosts)
}

//根据Uri获取连接
func (self *MoaClientManager) SelectClient(uri string, key string) (*tclient.RemotingClient, error) {

	self.lock.RLock()
	defer self.lock.RUnlock()

	strategy, ok := self.uri2Ips[uri]
	if ok {
		ip := strategy.Select(key)
		if len(ip) > 0 {
			p := self.clientsManager.FindRemoteClient(ip)
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
func (self *MoaClientManager) packetDis(c *tclient.RemotingClient, p *packet.Packet) {
	if p.Header.CmdType == proto.PONG {
		pipo := p.PayLoad.(proto.PiPo)
		c.Attach(p.Header.Opaque, pipo.Timestamp)
	} else {
		c.Attach(p.Header.Opaque, p.PayLoad)
	}
}
