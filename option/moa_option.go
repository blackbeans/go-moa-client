package option

import (
	"errors"
	"github.com/blackbeans/go-moa-client/client/hash"
	log "github.com/blackbeans/log4go"
	"github.com/naoina/toml"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

type HostPort struct {
	Hosts string
}

//配置信息
type Option struct {
	Env struct {
		RunMode          string
		BindAddress      string
		RegistryType     string
		AppName          string
		AppSecretKey     string
		ServiceUriSuffix string
	}

	//使用的环境
	Registry map[string]HostPort //momokeeper的配置
	Clusters map[string]Cluster  //各集群的配置
}

//----------------------------------------
//Cluster配置
type Cluster struct {
	Env              string //当前环境使用的是dev还是online
	ProcessTimeout   int    //处理超时 5 s单位
	PoolSizePerHost  int    //5
	LogFile          string //log4go的文件路径
	SelectorStrategy string //客户端选择连接方式 ketama/random 默认random
}

//---------最终需要的ClientCOption
type ClientOption struct {
	AppName          string
	AppSecretKey     string
	RegistryType     string
	RegistryHosts    string
	ProcessTimeout   time.Duration
	PoolSizePerHost  int
	ServiceUriSuffix string
	SelectorStrategy hash.StrategyType
}

func LoadConfiruation(path string) (*ClientOption, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buff, rerr := ioutil.ReadAll(f)
	if nil != rerr {
		return nil, rerr
	}
	// log.DebugLog("application", "LoadConfiruation|Parse|toml:%s", string(buff))
	//读取配置
	var option Option
	err = toml.Unmarshal(buff, &option)
	if nil != err {
		return nil, err
	}

	cluster, ok := option.Clusters[option.Env.RunMode]
	if !ok {
		return nil, errors.New("no cluster config for " + option.Env.RunMode)
	}

	//加载log4go
	log.LoadConfiguration(cluster.LogFile)

	reg, exist := option.Registry[option.Env.RunMode]
	if !exist {
		return nil, errors.New("no reg  for " + option.Env.RunMode + ":" + cluster.Env)
	}

	//拼装为可用的MOA参数
	mop := &ClientOption{}
	mop.ServiceUriSuffix = option.Env.ServiceUriSuffix
	mop.AppName = option.Env.AppName
	mop.AppSecretKey = option.Env.AppSecretKey
	mop.RegistryType = option.Env.RegistryType
	mop.RegistryHosts = reg.Hosts
	mop.ProcessTimeout = time.Duration(int64(cluster.ProcessTimeout) * int64(time.Second))
	mop.PoolSizePerHost = cluster.PoolSizePerHost

	strategy := hash.STRATEGY_RANDOM
	switch strings.ToUpper(cluster.SelectorStrategy) {
	case "KETAMA":
		strategy = hash.STRATEGY_KETAMA
	case "RANDOM":
		fallthrough
	default:
		strategy = hash.STRATEGY_RANDOM
	}
	mop.SelectorStrategy = hash.StrategyType(strategy)
	return mop, nil

}
