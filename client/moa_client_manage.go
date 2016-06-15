package client

import (
	"errors"
	"fmt"
	"github.com/blackbeans/go-moa-client/option"
	"github.com/blackbeans/go-moa/lb"
	log "github.com/blackbeans/log4go"

	"gopkg.in/redis.v3"
	"math/rand"

	"strings"
	"sync"
	"time"
)

type MoaClientManager struct {
	uri2Pool   map[string] /*uri*/ []*redis.Client
	ip2Options map[string] /*ip:port*/ *redis.Options
	ip2Client  map[string] /**ip:port*/ *redis.Client
	uri2Ips    map[string] /*uri*/ []string /*ip:port*/

	addrManager *AddressManager
	op          *option.ClientOption
	lock        sync.RWMutex
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

	manager := &MoaClientManager{}
	manager.op = op
	manager.uri2Pool = make(map[string] /*uri*/ []*redis.Client, 10)
	manager.ip2Options = make(map[string] /*ip:port*/ *redis.Options, 10)
	manager.ip2Client = make(map[string] /**ip:port*/ *redis.Client, 10)
	manager.uri2Ips = make(map[string][]string /*ip:port*/, 2)
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
		log.InfoLog("address_manager", "MoaClientManager|Create Client|SUCC|%s", hp)
	}

	//开始执行新增连接和删除操作
	self.lock.Lock()
	//add first
	for op, c := range addOp2Clients {

		self.ip2Client[op.Addr] = c

		self.ip2Options[op.Addr] = op

		log.InfoLog("address_manager", "MoaClientManager|Store Client|SUCC|%s", op.Addr)
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
	self.uri2Ips[uri] = hosts
	log.InfoLog("address_manager", "MoaClientManager|Store Uri Pool|SUCC|%s|%d", uri, len(newPool))
	//清理掉不再使用redisClient
	usingIps := make(map[string]bool, 5)
	for _, v := range self.uri2Ips {
		for _, ip := range v {
			usingIps[ip] = true
		}
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
			log.WarnLog("address_manager", "MoaClientManager|Remove Expired Client|%s", hp)
		}
	}
	self.lock.Unlock()

	log.InfoLog("address_manager", "MoaClientManager|OnAddressChange|SUCC|%s|%v", uri, hosts)
}

//根据Uri获取连接
func (self MoaClientManager) SelectClient(uri string) (*redis.Client, error) {

	self.lock.RLock()
	defer self.lock.RUnlock()
	pool, ok := self.uri2Pool[uri]
	if ok {
		p := pool[rand.Intn(len(pool))]
		return p, nil

	} else {
		return nil, errors.New(fmt.Sprintf("NO CLIENT FOR %s", uri))
	}

}

func (self MoaClientManager) Destory() {
	for _, c := range self.ip2Client {
		c.Close()
	}
}
