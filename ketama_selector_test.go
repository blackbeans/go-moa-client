package client

import (
	"testing"
)

func TestRandom(t *testing.T) {
	nodes := []string{"localhost:2181", "localhost:2182", "localhost:2183"}
	strategy := NewRandomStrategy(nodes)
	host := strategy.Select("100777")
	t.Log(host)
	if host != "localhost:2182" {
		t.Fail()
	}

	host = strategy.Select("100778")
	t.Log(host)
	if host != "localhost:2181" {
		t.Fail()
	}

	//change
	nodes = []string{"localhost:2186"}
	strategy.ReHash(nodes)
	host = strategy.Select("100777")
	t.Log(host)
	if host != "localhost:2186" {
		t.Fail()
	}
}

func BenchmarkRandomStrategy_Select(b *testing.B) {
	nodes := []string{"localhost:2181", "localhost:2182", "localhost:2183"}
	strategy := NewRandomStrategy(nodes)
	count_1 := 0
	count_2 := 0
	count_3 := 0
	for i := 0; i < b.N; i++ {
		host := strategy.Select("100777")
		if host == "localhost:2181" {
			count_1++
		} else if host == "localhost:2182" {
			count_2++
		} else if host == "localhost:2183" {
			count_3++
		}
	}
	b.Logf("%d,%d,%d", count_1, count_2, count_3)
}

func TestKetamaSelector(t *testing.T) {

	nodes := []string{"localhost:2181", "localhost:2182", "localhost:2183"}
	strategy := NewKetamaStrategy(nodes)
	host := strategy.Select("100777")
	t.Log(host)
	if host != "localhost:2182" {
		t.Fail()
	}

	host = strategy.Select("100778")
	t.Log(host)
	if host != "localhost:2181" {
		t.Fail()
	}

	//change
	nodes = []string{"localhost:2186"}
	strategy.ReHash(nodes)
	host = strategy.Select("100777")
	t.Log(host)
	if host != "localhost:2186" {
		t.Fail()
	}
}

func BenchmarkKetamaSelector(b *testing.B) {
	nodes := []string{"localhost:2181", "localhost:2182", "localhost:2183"}
	strategy := NewKetamaStrategy(nodes)
	for i := 0; i < b.N; i++ {
		host := strategy.Select("100777")
		if host != "localhost:2182" {
			b.Fail()
		}
	}
}
