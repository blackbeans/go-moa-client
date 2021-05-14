package client

import (
	core "github.com/blackbeans/go-moa"
	"math/rand"
	"sync"
	"time"
)

type RandomStrategy struct {
	nodes  []core.ServiceMeta
	length int
	sync.RWMutex
}

func NewRandomStrategy(nodes []core.ServiceMeta) *RandomStrategy {
	return &RandomStrategy{
		nodes:  nodes,
		length: len(nodes)}
}

func (self *RandomStrategy) ReHash(nodes []core.ServiceMeta) {
	self.Lock()
	defer self.Unlock()
	self.nodes = nodes
	self.length = len(nodes)
}

func (self *RandomStrategy) Select(key string) core.ServiceMeta {
	self.RLock()
	defer self.RUnlock()
	if len(self.nodes) <= 0 {
		return core.ServiceMeta{}
	}
	src := rand.NewSource(time.Now().UnixNano())
	random := rand.New(src)
	return self.nodes[random.Intn(self.length)]
}

func (self *RandomStrategy) Iterator(f func(idx int, node core.ServiceMeta)) {
	self.RLock()
	defer self.RUnlock()
	for i, n := range self.nodes {
		f(i, n)
	}
}

func (RandomStrategy) NegativeFeedback(core.ServiceMeta) {}