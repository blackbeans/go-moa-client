package client

import (
	"sort"
	"sync"
	"time"

	"github.com/blackbeans/go-moa/lb"
	log "github.com/blackbeans/log4go"
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
	hosts := center.loadAvaiableAddress()
	center.uri2Hosts = hosts
	go func() {
		for {
			time.Sleep(5 * time.Second)
			func() {
				defer func() {
					if err := recover(); nil != err {

					}
				}()
				//需要定时拉取服务地址
				hosts := center.loadAvaiableAddress()
				center.lock.Lock()
				center.uri2Hosts = hosts
				center.lock.Unlock()
			}()
		}
	}()

	return center
}

func (self AddressManager) loadAvaiableAddress() map[string][]string {
	hosts := make(map[string][]string, 2)
	for _, uri := range self.serviceUris {
		for i := 0; i < 3; i++ {
			serviceUri, groupId := splitServiceUri(uri)
			addrs, err := self.registry.GetService(serviceUri, lb.PROTOCOL, groupId)
			if nil != err {
				log.WarnLog("config_center", "AddressManager|loadAvaiableAddress|FAIL|%s|%s", err, uri)
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
						if len(oldAddrs) > 0 &&
							len(oldAddrs) == len(addrs) {
							for j, v := range addrs {
								if oldAddrs[j] != v {
									needChange = true
									break
								}
							}
						} else {
							needChange = true
						}
					} else {
						needChange = true
					}

					//变化通知
					if needChange {
						log.InfoLog("config_center",
							"AddressManager|loadAvaiableAddress|NeedChange|%s|old:%v|news:%v", uri, oldAddrs, addrs)
						self.listener(uri, addrs)

					}
				}()

				break
			}
		}
	}

	return hosts

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
