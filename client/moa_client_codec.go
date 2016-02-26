package client

import (
	"bytes"
	"git.wemomo.com/bibi/go-moa/protocol"
	"github.com/blackbeans/turbo/packet"
	"strconv"
)

const (
	GET_PREFIX = "*2\r\n$3\r\nGET\r\n"
)

type MoaClientCodeC struct {
	protocol.RedisGetCodec
}

//序列化
//直接获取data
//*+2+\r\n$3\r\nGET\r\n$n\r\n[data]+\r\n
func (self MoaClientCodeC) MarshalPacket(packet *packet.Packet) []byte {
	body := string(packet.Data)
	l := len(strconv.Itoa(len(body)))
	buff := bytes.NewBuffer(make([]byte, 0, len(GET_PREFIX)+1+l+2+len(body)+2))
	buff.WriteString(GET_PREFIX)
	buff.WriteString("$")
	buff.WriteString(strconv.Itoa(len(body)))
	buff.WriteString("\r\n")
	buff.WriteString(body)
	buff.WriteString("\r\n")
	return buff.Bytes()
}
