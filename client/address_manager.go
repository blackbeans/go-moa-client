package client

import (
	"git.wemomo.com/bibi/go-moa/lb"
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
		registry: registry, uri2Hosts: uri2Hosts}
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
			} else {
				if len(addrs) > 0 {
					sort.Strings(addrs)
					hosts[uri] = addrs
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
					needChange := false
					oldAddrs, ok := self.uri2Hosts[uri]
					if ok {

						if len(oldAddrs) != len(addrs) {
							needChange = true
						} else {
							for j, v := range addrs {
								if oldAddrs[j] != v {
									needChange = true
									break
								}
							}
						}
					} else {
						//旧的里面没有数据，判断新的addrs如果有数据说明新增需要变化
						if len(oldAddrs) != len(addrs) {
							needChange = true
						}
					}
					//变化通知
					if needChange {
						self.listener(uri, addrs)
						log.InfoLog("address_manager", "AddressManager|loadAvaiableAddress|listener|SUCC|%s|%s", uri, addrs)
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
