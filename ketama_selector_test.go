package client

import (
	"testing"
	"time"

	core "github.com/blackbeans/go-moa"
)

func TestRandom(t *testing.T) {
	nodes := []core.ServiceMeta{
		core.ServiceMeta{HostPort: "localhost:2181"},
		core.ServiceMeta{HostPort: "localhost:2182"},
		core.ServiceMeta{HostPort: "localhost:2183"}}
	strategy := NewRandomStrategy(nodes)
	host := strategy.Select("100777")
	t.Log(host)
	if host.HostPort != "localhost:2182" {
		t.Fail()
	}

	host = strategy.Select("100778")
	t.Log(host)
	if host.HostPort != "localhost:2181" {
		t.Fail()
	}

	//change
	nodes = []core.ServiceMeta{core.ServiceMeta{HostPort: "localhost:2186"}}
	strategy.ReHash(nodes)
	host = strategy.Select("100777")
	t.Log(host)
	if host.HostPort != "localhost:2186" {
		t.Fail()
	}
}

func BenchmarkRandomStrategy_Select(b *testing.B) {
	nodes := []core.ServiceMeta{
		core.ServiceMeta{HostPort: "localhost:2181"},
		core.ServiceMeta{HostPort: "localhost:2182"},
		core.ServiceMeta{HostPort: "localhost:2183"}}
	strategy := NewRandomStrategy(nodes)
	count_1 := 0
	count_2 := 0
	count_3 := 0
	for i := 0; i < b.N; i++ {
		host := strategy.Select("100777")
		if host.HostPort == "localhost:2181" {
			count_1++
		} else if host.HostPort == "localhost:2182" {
			count_2++
		} else if host.HostPort == "localhost:2183" {
			count_3++
		}
	}
	b.Logf("%d,%d,%d", count_1, count_2, count_3)
}

func TestKetamaSelector(t *testing.T) {

	nodes := []core.ServiceMeta{
		core.ServiceMeta{HostPort: "localhost:2181"},
		core.ServiceMeta{HostPort: "localhost:2182"},
		core.ServiceMeta{HostPort: "localhost:2183"}}
	strategy := NewKetamaStrategy(nodes)
	host := strategy.Select("100777")
	t.Log(host)
	if host.HostPort != "localhost:2182" {
		t.Fail()
	}

	host = strategy.Select("100778")
	t.Log(host)
	if host.HostPort != "localhost:2181" {
		t.Fail()
	}

	//change
	nodes = []core.ServiceMeta{
		core.ServiceMeta{HostPort: "localhost:2186"}}
	strategy.ReHash(nodes)
	host = strategy.Select("100777")
	t.Log(host)
	if host.HostPort != "localhost:2186" {
		t.Fail()
	}
}

func BenchmarkKetamaSelector(b *testing.B) {
	nodes := []core.ServiceMeta{
		core.ServiceMeta{HostPort: "localhost:2181"},
		core.ServiceMeta{HostPort: "localhost:2182"},
		core.ServiceMeta{HostPort: "localhost:2183"}}
	strategy := NewKetamaStrategy(nodes)
	for i := 0; i < b.N; i++ {
		host := strategy.Select("100777")
		if host.HostPort != "localhost:2182" {
			b.Fail()
		}
	}
}


func TestKetamaPriorityRandomStrategy(t *testing.T) {
	nodes := []core.ServiceMeta{
		{HostPort: "localhost:2181"},
		{HostPort: "localhost:2182"},
		{HostPort: "localhost:2183"}}

	strategy := NewPriorityRandomStrategy(nodes)


	// 优先级测试 和 ReHash
	nodes = []core.ServiceMeta{
		{HostPort: "localhost:2187"},
		{HostPort: "localhost:2188"},
		{HostPort: "localhost:2189"}}
	

	strategy.ReHash(nodes)
	host1 := strategy.Select("100777")
	t.Log(host1)
	// 降低优先级 到 0
	for i := 0; i<100; i++ {
		strategy.NegativeFeedback(host1)
	}

	// 10s 内能运行完
	for i := 0; i< 1000; i++{
		host2 := strategy.Select("100777")
		if host2.HostPort == host1.HostPort {
			t.Fatal(host1, host2)
		}
	}

	// 等待 11s 其恢复 1点优先级
	time.Sleep(11 * time.Second)
	// 1000 次总得至少中一次吧
	for i := 0; i< 1000; i++{
		host2 := strategy.Select("100777")
		if host2.HostPort == host1.HostPort {
			t.Logf("命中，i：%d", i)
		}
	}
}