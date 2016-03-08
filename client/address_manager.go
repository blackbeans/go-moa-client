package client

import (
	"bytes"
	"git.wemomo.com/bibi/go-moa/lb"
	"github.com/blackbeans/go-zookeeper/zk"
	log "github.com/blackbeans/log4go"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	PROTOCOL_TYPE = "redis"
)

const (
	ZK_MOA_ROOT_PATH  = "/go-moa/services"
	ZK_ROOT           = "/"
	ZK_PATH_DELIMITER = "/"
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
	registryType := reflect.TypeOf(registry).String()

	if strings.Contains(registryType, REGISTRY_TYPE_MOMOKEEPER) {
		// momokeeper 需要定时轮训
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
	} else if strings.Contains(registryType, REGISTRY_TYPE_ZOOKEEPER) {
		// zookeeper 坐等服务端通知即可
		// for _, uri := range uris {
		// 	regZk, _ := registry.(lb.zookeeper)
		// 	conn := regZk.GetRegConn()
		// 	path := concat(ZK_MOA_ROOT_PATH, concat(uri, "_", PROTOCOL_TYPE))
		// 	func() {
		// 		defer func() {
		// 			if err := recover(); nil != err {

		// 			}
		// 		}()
		// 		//需要定时拉取服务地址
		// 		center.listenZkNodeChanged(conn, path)
		// 	}()
		// }
	} else {
		log.ErrorLog("address_manager", "AddressManager|can't support this registryType|%s", registryType)
	}

	return center
}

func (self AddressManager) listenZkNodeChanged(conn *zk.Conn, path, uri string) error {
	snapshots, errors := mirror(conn, path)
	go func() {
		for {
			select {
			case addrs := <-snapshots:
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
			case err := <-errors:
				panic(err)
			}
		}
	}()
	return nil
}

func mirror(conn *zk.Conn, path string) (chan []string, chan error) {
	snapshots := make(chan []string)
	errors := make(chan error)
	go func() {
		for {
			snapshot, _, events, err := conn.ChildrenW(path)
			if err != nil {
				errors <- err
				return
			}
			snapshots <- snapshot
			evt := <-events
			if evt.Err != nil {
				errors <- evt.Err
				return
			}
		}
	}()
	return snapshots, errors
}

// 拼接字符串
func concat(args ...string) string {
	var buffer bytes.Buffer
	for _, arg := range args {
		buffer.WriteString(arg)
	}
	return buffer.String()
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
