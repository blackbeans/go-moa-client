package client

import (
	"github.com/blackbeans/go-moa/lb"
	log "github.com/blackbeans/log4go"
	"sort"
	"sync"
	"time"
)

const (
	PROTOCOL_TYPE = "redis"
)

type IAddressListener func(uri string, hosts []string)

type AddressManager struct {
	serviceUris []string
	registry    lb.IRegistry
	uri2Hosts   map[string][]string
	lock        sync.RWMutex
	listener    IAddressListener
}

func NewAddressManager(registry lb.IRegistry, uris []string, listener IAddressListener) *AddressManager {

	uri2Hosts := make(map[string][]string, 2)
	center := &AddressManager{serviceUris: uris,
		registry: registry, uri2Hosts: uri2Hosts, listener: listener}
	center.loadAvaiableAddress()
	go func() {
		for {
			time.Sleep(5 * time.Second)
			func() {
				defer func() {
					if err := recover(); nil != err {

					}
				}()
				//需要定时拉取服务地址
				center.loadAvaiableAddress()
			}()
		}
	}()

	return center
}

func (self AddressManager) loadAvaiableAddress() {
	hosts := make(map[string][]string, 2)
	for _, uri := range self.serviceUris {
		for i := 0; i < 3; i++ {
			addrs, err := self.registry.GetService(uri, PROTOCOL_TYPE)
			if nil != err {
				log.WarnLog("address_manager", "AddressManager|loadAvaiableAddress|FAIL|%s|%s", err, uri)
				func() {
					self.lock.RLock()
					defer self.lock.RUnlock()
					oldAddrs, ok := self.uri2Hosts[uri]
					if ok {
						hosts[uri] = oldAddrs
					}
				}()
			} else {
				if len(addrs) > 0 {
					sort.Strings(addrs)
					hosts[uri] = addrs
					log.InfoLog("address_manager", "AddressManager|loadAvaiableAddress|GetService|SUCC|%s|%s", uri, addrs)
				}
				//对比变化
				func() {

					defer func() {
						if r := recover(); nil != r {
							//do nothing
						}
					}()
					self.lock.RLock()
					defer self.lock.RUnlock()
					needChange := true
					oldAddrs, ok := self.uri2Hosts[uri]
					if ok {
						if len(oldAddrs) > 0 &&
							len(oldAddrs) == len(addrs) {
							for j, v := range addrs {
								//如果是最后一个并且相等那么就应该不需要更新
								if oldAddrs[j] == v && j == len(addrs)-1 {
									needChange = false
									break
								}
							}

						}
					}
					//变化通知
					if needChange {
						self.listener(uri, addrs)
					}
				}()

				break
			}
		}
	}

	self.lock.Lock()
	self.uri2Hosts = hosts
	self.lock.Unlock()
}

var EMPTY_HOSTS = []string{}

func (self AddressManager) GetService(serviceUri string) []string {
	self.lock.RLock()
	defer self.lock.RUnlock()
	hosts, ok := self.uri2Hosts[serviceUri]
	if !ok {
		return EMPTY_HOSTS
	} else {
		return hosts
	}
}
