package client

import (
	core "github.com/blackbeans/go-moa"
	"sync"
)

type Strategy interface {
	Select(key string) core.ServiceMeta
	ReHash(nodes []core.ServiceMeta)
	Iterator(f func(idx int, node core.ServiceMeta))
}

type KetamaStrategy struct {
	ketama *Ketama
	Nodes  []core.ServiceMeta
	sync.RWMutex
}

func NewKetamaStrategy(services []core.ServiceMeta) *KetamaStrategy {
	ketama := NewKetama(services, len(services)*2)
	return &KetamaStrategy{
		ketama: ketama,
		Nodes:  services}
}

func (self *KetamaStrategy) ReHash(nodes []core.ServiceMeta) {
	self.Lock()
	defer self.Unlock()
	ketama := NewKetama(nodes, len(nodes)*2)
	self.ketama = ketama
	self.Nodes = nodes
}

func (self *KetamaStrategy) Select(key string) core.ServiceMeta {
	self.RLock()
	defer self.RUnlock()
	n := self.ketama.Node(key)
	return n
}

func (self *KetamaStrategy) Iterator(f func(idx int, node core.ServiceMeta)) {
	self.RLock()
	defer self.RUnlock()
	for i, n := range self.Nodes {
		f(i, n)
	}
}
