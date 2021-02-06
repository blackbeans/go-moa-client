package client

import (
	"fmt"
	core "github.com/blackbeans/go-moa"
	"sort"
)

// Convenience types for common cases
// UIntSlice attaches the methods of Interface to []uint, sorting in increasing order.
type UIntSlice []uint

func (p UIntSlice) Len() int           { return len(p) }
func (p UIntSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p UIntSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type Ketama struct {
	node         int                       // phsical node number
	vnode        int                       // virual node number
	nodes        []uint                    // nodes
	nodesMapping map[uint]core.ServiceMeta // nodes maping
}

func NewKetama(node []core.ServiceMeta, vnode int) *Ketama {
	ketama := &Ketama{}
	ketama.node = len(node)
	ketama.vnode = vnode
	ketama.nodes = []uint{}
	ketama.nodesMapping = map[uint]core.ServiceMeta{}

	ketama.initCircle(node)
	return ketama
}

// init consistent hashing circle
func (k *Ketama) initCircle(nodes []core.ServiceMeta) {
	h := NewMurmur3C()
	for _, n := range nodes {
		for i := 0; i < k.vnode; i++ {
			vnode := fmt.Sprintf("%s#%d", n.HostPort, i)
			h.Write([]byte(vnode))
			vpos := uint(h.Sum32())
			k.nodes = append(k.nodes, vpos)
			k.nodesMapping[vpos] = n
			h.Reset()
		}
	}

	sort.Sort(UIntSlice(k.nodes))
}

// Node get a consistent hashing node by key
func (k *Ketama) Node(key string) core.ServiceMeta {
	if len(k.nodes) == 0 {
		return core.ServiceMeta{}
	}
	h := NewMurmur3C()
	h.Write([]byte(key))
	idx := searchLeft(k.nodes, uint(h.Sum32()))
	pos := k.nodes[0]
	if idx != len(k.nodes) {
		pos = k.nodes[idx]
	}

	return k.nodesMapping[pos]
}

// search the slice left-most value
func searchLeft(a []uint, x uint) int {
	lo := 0
	hi := len(a)
	for lo < hi {
		mid := (lo + hi) / 2
		if a[mid] < x {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	return lo
}
