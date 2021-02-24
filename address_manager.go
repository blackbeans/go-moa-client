package client

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/blackbeans/go-moa"

	log "github.com/blackbeans/log4go"
)

type IAddressListener func(uri string, hosts []core.ServiceMeta)

type AddressManager struct {
	serviceUris  []string
	registry     core.IRegistry
	uri2Services map[string][]core.ServiceMeta
	lock         sync.RWMutex
	listener     IAddressListener
}

func NewAddressManager(registry core.IRegistry, uris []string, listener IAddressListener) *AddressManager {

	uri2Services := make(map[string][]core.ServiceMeta, 2)
	center := &AddressManager{serviceUris: uris,
		registry:     registry,
		uri2Services: uri2Services,
		listener:     listener}

	center.uri2Services = uri2Services
	//需要定时拉取服务地址
	hosts := center.loadAvailableAddress()
	center.lock.Lock()
	center.uri2Services = hosts
	center.lock.Unlock()
	go func() {
		for {
			time.Sleep(5 * time.Second)
			func() {
				defer func() {
					if err := recover(); nil != err {

					}
				}()
				//需要定时拉取服务地址
				hosts := center.loadAvailableAddress()
				center.lock.Lock()
				center.uri2Services = hosts
				center.lock.Unlock()
			}()
		}
	}()

	return center
}

func (self *AddressManager) loadAvailableAddress() map[string][]core.ServiceMeta {
	hosts := make(map[string][]core.ServiceMeta, 2)
	for _, uri := range self.serviceUris {
		for i := 0; i < 3; i++ {
			serviceUri, groupId := core.UnwrapServiceUri(uri)
			addrs, err := self.registry.GetService(serviceUri, core.PROTOCOL, groupId)
			if nil != err {
				log.WarnLog("config_center", "AddressManager|loadAvailableAddress|FAIL|%s|%s", err, uri)
				self.lock.RLock()
				oldAddrs, ok := self.uri2Services[uri]
				if ok {
					hosts[uri] = oldAddrs
				}
				self.lock.RUnlock()
			} else {

				if len(addrs) > 0 {
					sort.SliceIsSorted(addrs, func(i, j int) bool {
						if strings.Compare(addrs[i].HostPort, addrs[j].HostPort) > 0 {
							return true
						}
						return false
					})
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
					oldAddrs, ok := self.uri2Services[uri]
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
							"AddressManager|loadAvailableAddress|NeedChange|%s|old:%v|news:%v", uri, oldAddrs, addrs)
						self.listener(uri, addrs)

					}
				}()

				break
			}
		}
	}

	return hosts

}

var EMPTY_HOSTS = []core.ServiceMeta{}

func (self *AddressManager) GetService(serviceUri string) []core.ServiceMeta {
	self.lock.RLock()
	defer self.lock.RUnlock()
	hosts, ok := self.uri2Services[serviceUri]
	if !ok {
		return EMPTY_HOSTS
	} else {
		return hosts
	}
}
