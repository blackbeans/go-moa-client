package client

import (
	"math/rand"
	"sync"
	"time"

	core "github.com/blackbeans/go-moa"
)

// PriorityRandomStrategy 优先级随机选择
type PriorityRandomStrategy struct {
	nodes    []core.ServiceMeta
	priority map[core.ServiceMeta]int // 优先级
	length   int
	random   *rand.Rand
	sync.RWMutex
}

func NewPriorityRandomStrategy(nodes []core.ServiceMeta) *PriorityRandomStrategy {
	p := make(map[core.ServiceMeta]int, len(nodes))
	for _, s := range nodes {
		p[s] = 100
	}

	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	priorityRandomStrategy := &PriorityRandomStrategy{
		nodes:    nodes,
		priority: p,
		random:   random,
		length:   len(nodes),
	}

	// 每10秒恢复1点优先级
	go func() {
		timer := time.NewTicker(10 * time.Second)
		defer timer.Stop()

		for {
			<-timer.C
			priorityRandomStrategy.Lock()
			for s, priority := range priorityRandomStrategy.priority {
				if priority == 100 {
					continue
				}
				priorityRandomStrategy.priority[s]++
			}
			priorityRandomStrategy.Unlock()
		}
	}()

	return priorityRandomStrategy
}

func (self *PriorityRandomStrategy) ReHash(nodes []core.ServiceMeta) {
	self.Lock()
	defer self.Unlock()
	self.nodes = nodes
	self.length = len(nodes)
	p := make(map[core.ServiceMeta]int, len(nodes))
	for _, s := range nodes {
		p[s] = 100
	}
	self.priority = p
}

func (self *PriorityRandomStrategy) Select(key string) core.ServiceMeta {
	self.RLock()
	defer self.RUnlock()
	if len(self.nodes) <= 0 {
		return core.ServiceMeta{}
	}

	// 优先级越高
	var sum int
	for _, v := range self.priority {
		sum += v
	}
	n := self.random.Intn(sum)
	for s, v := range self.priority {
		n = n - v
		if n < 0 {
			return s
		}
	}

	return self.nodes[self.random.Intn(self.length)]
}

func (self *PriorityRandomStrategy) Iterator(f func(idx int, node core.ServiceMeta)) {
	self.RLock()
	defer self.RUnlock()
	for i, n := range self.nodes {
		f(i, n)
	}
}

// NegativeFeedback 负反馈，调用发生错，减小其优先级
func (self *PriorityRandomStrategy) NegativeFeedback(s core.ServiceMeta) {
	if self.priority[s] <= 0 {
		self.priority[s] = 0
		return
	}

	self.Lock()
	defer self.Unlock()
	if self.priority[s] <= 0 {
		self.priority[s] = 0
		return
	}
	self.priority[s]--
}
