package client

import (
	"errors"
	"fmt"
	"github.com/blackbeans/go-moa-client/client/hash"
	"github.com/blackbeans/go-moa/core"
	"github.com/blackbeans/go-moa/lb"
	log "github.com/blackbeans/log4go"
	"gopkg.in/redis.v5"
	"strings"
	"sync"
	"time"
)

type MoaClientManager struct {
	uri2Pool   map[string] /*uri*/ []*redis.Client
	ip2Options map[string] /*ip:port*/ *redis.Options
	ip2Client  map[string] /**ip:port*/ *redis.Client
	// []string /*ip:port*/
	uri2Ips map[string] /*uri*/ hash.Strategy

	addrManager *AddressManager
	op          *ClientOption
	lock        sync.RWMutex
}

func NewMoaClientManager(op *ClientOption, uris []string) *MoaClientManager {
	var reg lb.IRegistry
	if strings.HasPrefix(op.RegistryHosts, core.SCHEMA_ZK) {
		reg = lb.NewZkRegistry(strings.TrimPrefix(op.RegistryHosts, core.SCHEMA_ZK), uris, false)
	}

	manager := &MoaClientManager{}
	manager.op = op
	manager.uri2Pool = make(map[string] /*uri*/ []*redis.Client, 10)
	manager.ip2Options = make(map[string] /*ip:port*/ *redis.Options, 10)
	manager.ip2Client = make(map[string] /**ip:port*/ *redis.Client, 10)
	manager.uri2Ips = make(map[string]hash.Strategy, 2)
	addrManager := NewAddressManager(reg, uris, manager.OnAddressChange)
	manager.addrManager = addrManager

	return manager
}

func (self MoaClientManager) OnAddressChange(uri string, hosts []string) {
	//需要移除的连接
	removeHostport := make([]string, 0, 2)
	//新增地址
	addHostport := make([]string, 0, 2)

	self.lock.RLock()
	//寻找新增连接
	for _, ip := range hosts {

		_, ok := self.ip2Options[ip]
		if !ok {
			addHostport = append(addHostport, ip)
		}
	}

	self.lock.RUnlock()

	//新增连接
	addOp2Clients := make(map[*redis.Options]*redis.Client, 10)
	for _, hp := range addHostport {
		op := &redis.Options{
			Addr:        hp,
			Password:    "", // no password set
			DB:          0,  // use default DB
			PoolSize:    self.op.PoolSizePerHost,
			PoolTimeout: 10 * time.Minute,
			ReadTimeout: self.op.ProcessTimeout}
		//创建redis的实例
		c := redis.NewClient(op)
		addOp2Clients[op] = c
		log.InfoLog("config_center", "MoaClientManager|Create Client|SUCC|%s", hp)
	}

	//开始执行新增连接和删除操作
	self.lock.Lock()
	//add first
	for op, c := range addOp2Clients {

		self.ip2Client[op.Addr] = c

		self.ip2Options[op.Addr] = op

		log.InfoLog("config_center", "MoaClientManager|Store Client|SUCC|%s", op.Addr)
	}

	//重新构建uri对应的连接组
	newPool := make([]*redis.Client, 0, 5)
	for _, ip := range hosts {
		c, ok := self.ip2Client[ip]
		if ok {
			newPool = append(newPool, c)
		}

	}
	self.uri2Pool[uri] = newPool

	if self.op.SelectorStrategy == hash.STRATEGY_KETAMA {
		self.uri2Ips[uri] = hash.NewKetamaStrategy(hosts)
	} else if self.op.SelectorStrategy == hash.STRATEGY_RANDOM {
		self.uri2Ips[uri] = hash.NewRandomStrategy(hosts)
	} else {
		self.uri2Ips[uri] = hash.NewRandomStrategy(hosts)
	}

	log.InfoLog("config_center", "MoaClientManager|Store Uri Pool|SUCC|%s|%v|%d", uri, hosts, len(newPool))
	//清理掉不再使用redisClient
	usingIps := make(map[string]bool, 5)
	for _, v := range self.uri2Ips {
		v.Iterator(func(i int, ip string) {
			usingIps[ip] = true
		})
	}

	for ip := range self.ip2Options {
		_, ok := usingIps[ip]
		if !ok {
			//不再使用了移除
			removeHostport = append(removeHostport, ip)
		}
	}

	for _, hp := range removeHostport {
		delete(self.ip2Options, hp)
		c, ok := self.ip2Client[hp]
		if ok {
			c.Close()
			log.WarnLog("config_center", "MoaClientManager|Remove Expired Client|%s", hp)
		}
		delete(self.ip2Client, hp)
	}
	self.lock.Unlock()

	log.InfoLog("config_center", "MoaClientManager|OnAddressChange|SUCC|%s|%v", uri, hosts)
}

//根据Uri获取连接
func (self MoaClientManager) SelectClient(uri string, key string) (*redis.Client, error) {

	self.lock.RLock()
	defer self.lock.RUnlock()

	strategy, ok := self.uri2Ips[uri]
	if ok {
		ip := strategy.Select(key)
		p, yes := self.ip2Client[ip]
		if yes {
			return p, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("NO CLIENT FOR %s", uri))

}

func (self MoaClientManager) Destroy() {
	for _, c := range self.ip2Client {
		c.Close()
	}
}
