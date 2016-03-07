package client

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
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

//直接获取data
func (self MoaClientCodeC) Read(reader *bufio.Reader) (*bytes.Buffer, error) {

	line, isPrefix, err := reader.ReadLine()
	if nil != err {
		return nil, errors.New("Read Packet Err " + err.Error())
	}

	//$+n+\r\n+ [data]+\r\n
	start := bytes.HasPrefix(line, []byte{'$'})
	if start {
		//获取到共有多少个\r\n
		l, _ := strconv.ParseInt(string(line[1:]), 10, 64)
		length := int(l)
		if length >= self.MaxFrameLength {
			return nil, errors.New(fmt.Sprintf("Too Large Packet %d/%d", length, self.MaxFrameLength))
		}
		//读取数组长度和对应的值
		tmp := bytes.NewBuffer(make([]byte, 0, length))
		dl := 0
		for {
			line, isPrefix, err = reader.ReadLine()
			if nil != err {
				return nil, errors.New("Read Packet Err " + err.Error())
			}

			//如果超过了给定的长度则忽略
			if (dl + len(line)) > length {
				return nil, errors.New(fmt.Sprintf("Invalid Packet Data %d/%d ", (dl + len(line)), length))
			}

			//没有读取完这个命令的字节继续读取
			l, er := tmp.Write(line)
			if nil != err {
				return nil, errors.New("Write Packet Into Buff  Err " + er.Error())
			}
			//读取完这个命令的字节
			if !isPrefix {
				break
			} else {

			}
			dl += l
		}
		//得到了get和数据将数据返回出去
		return tmp, nil
	} else {
		return nil, errors.New("Error Packet Prototol Is Not Get Response " + string(line[:0]))
	}
}

//反序列化
//包装为packet，但是头部没有信息
func (self MoaClientCodeC) UnmarshalPacket(buff *bytes.Buffer) (*packet.Packet, error) {
	return packet.NewRespPacket(0, 0, buff.Bytes()), nil
}

//序列化
//直接获取data
//*+2+\r\n$3\r\nGET\r\n$n\r\n[data]+\r\n
func (self MoaClientCodeC) MarshalPacket(p *packet.Packet) []byte {

	body := string(p.Data)
	l := len(strconv.Itoa(len(body)))
	buff := bytes.NewBuffer(make([]byte, 0, len(GET_PREFIX)+1+l+2+len(body)+2))
	buff.WriteString(GET_PREFIX)
	buff.WriteString("$")
	buff.WriteString(strconv.Itoa(len(body)))
	buff.WriteString("\r\n")
	buff.WriteString(body)
	buff.WriteString("\r\n")
	b := buff.Bytes()
	return b
}
