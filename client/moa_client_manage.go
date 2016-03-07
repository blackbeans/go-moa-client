package client

import (
	"errors"
	"git.wemomo.com/bibi/go-moa-client/option"
	"git.wemomo.com/bibi/go-moa/lb"
	log "github.com/blackbeans/log4go"
	"github.com/blackbeans/turbo"
	"github.com/blackbeans/turbo/client"
	"github.com/blackbeans/turbo/codec"
	"github.com/blackbeans/turbo/packet"
	"math/rand"
	"net"
	"strings"
	"time"
)

type MoaClientManager struct {
	clientManager *client.ClientManager
	addrManager   *AddressManager
	rc            *turbo.RemotingConfig
	op            *option.ClientOption
}

func NewMoaClientManager(op *option.ClientOption, uris []string) *MoaClientManager {
	var reg lb.IRegistry
	if op.RegistryType == "momokeeper" {
		split := strings.Split(op.RegistryHosts, ",")
		if len(split) > 1 {
			reg = lb.NewMomokeeper(split[0], split[1])
		} else {
			reg = lb.NewMomokeeper(split[0], split[0])
		}

	} else if op.RegistryType == "zookeeper" {

	}

	//网络参数
	rc := turbo.NewRemotingConfig(
		op.AppName,
		op.PoolSizePerHost, 16*1024,
		16*1024, 10000, 10000,
		10*time.Minute, 160000)

	manager := &MoaClientManager{}
	//创建连接重连握手回调
	reconn := client.NewReconnectManager(true, 5*time.Second, 10,
		func(ga *client.GroupAuth, remoteClient *client.RemotingClient) (bool, error) {
			//moa中没有握手返回成功
			return true, nil
		})
	manager.op = op
	manager.rc = rc
	manager.clientManager = client.NewClientManager(reconn)

	addrManager := NewAddressManager(reg, uris, manager.OnAddressChange)
	manager.addrManager = addrManager

	return manager
}

func (self MoaClientManager) OnAddressChange(uri string, hosts []string) {
	removeHostport := make([]string, 0, 2)
	//创建连接
	remoteClients := self.clientManager.FindRemoteClients([]string{uri},
		func(groupId string, rc *client.RemotingClient) bool {
			exist := false
			hostport := rc.RemoteAddr()
			for _, h := range hosts {
				if strings.HasPrefix(h, hostport) {
					exist = true
					break
				}
			}
			//如果不在对应列表中则是需要删除的
			if !exist {
				removeHostport = append(removeHostport, hostport)
			}
			//不存在直接过滤
			return !exist
		})

	//新增地址
	addHostport := make([]string, 0, 2)
	clients, ok := remoteClients[uri]
	//如果不存在则新增
	if !ok || len(clients) <= 0 {
		//创建连接
		addHostport = hosts
	} else {
		//如果存在则不新增

		for _, h := range hosts {
			exist := false
			for _, c := range clients {
				if strings.HasPrefix(h, c.RemoteAddr()) {
					exist = true
					break
				}

			}
			//有新增的地址
			if !exist {
				addHostport = append(addHostport, h)
			}
		}
	}
	//新增创建
	for _, h := range addHostport {
		split := strings.Split(h, "?")
		conn, err := dial(split[0])
		if nil == err {

			//需要开发对应的codec
			cf := func() codec.ICodec {
				decoder := MoaClientCodeC{}
				decoder.MaxFrameLength = 32 * 1024
				return decoder
			}

			//创建连接
			remoteClient := client.NewRemotingClient(conn, cf, self.readDispatcher, self.rc)
			remoteClient.Start()
			auth := client.NewGroupAuth(uri, self.op.AppSecretKey)
			succ := self.clientManager.Auth(auth, remoteClient)
			log.InfoLog("moa-server", "MoaClientManager|OnAddressChange|Auth|SUCC|%s|%s|%v", uri, h, succ)
		} else {
			log.WarnLog("moa-server", "MoaClientManager|OnAddressChange|Auth|FAIL|%s|%s|%s", err, uri, h)
		}

	}
}

//需要开发对应的分包
func (self MoaClientManager) readDispatcher(remoteClient *client.RemotingClient, p *packet.Packet) {
	//直接写过去[]byte结构
	//log.DebugLog("moa-server", "MoaClientManager|readDispatcher|%s", *p)
	remoteClient.Attach(p.Header.Opaque, p.Data)
}

//根据Uri获取连接
func (self MoaClientManager) SelectClient(uri string) (*client.RemotingClient, error) {
	remoteClients := self.clientManager.FindRemoteClients([]string{uri},
		func(groupId string, rc *client.RemotingClient) bool {
			return false
		})

	clients, ok := remoteClients[uri]
	if !ok || len(clients) <= 0 {
		return nil, errors.New("NO Client for [" + uri + "]")
	} else {
		return clients[rand.Intn(len(clients))], nil
	}
}

//创建物理连接
func dial(hostport string) (*net.TCPConn, error) {
	//连接
	remoteAddr, err_r := net.ResolveTCPAddr("tcp4", hostport)
	if nil != err_r {
		log.ErrorLog("address_manager", "MoaClientManager|DIAL|RESOLVE ADDR |FAIL|remote:%s\n", err_r)
		return nil, err_r
	}
	conn, err := net.DialTCP("tcp4", nil, remoteAddr)
	if nil != err {
		log.ErrorLog("address_manager", "MoaClientManager|DIAL|%s|FAIL|%s\n", hostport, err)
		return nil, err
	}

	return conn, nil
}
