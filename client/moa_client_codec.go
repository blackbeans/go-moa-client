package client

import (
	"bufio"
	"bytes"
	b "encoding/binary"
	"errors"
	"fmt"
	"github.com/blackbeans/go-moa/protocol"
	"github.com/blackbeans/turbo/packet"
	"strconv"
)

const (
	GET_PREFIX  = "*2\r\n$3\r\nGET\r\n"
	CMD_PADDING = 0x00
	CMD_GET     = 0x01
	CMD_PING    = 0x02
)

var PONG_BYTES []byte
var PING []byte
var PONG []byte
var DOLLAR []byte

func init() {

	DOLLAR = []byte{'$'}

	PING = []byte("*1\r\n$4\r\nPING\r\n")
	PONG = []byte("+PONG")

	tmp := bytes.NewBuffer(make([]byte, 0, 4+4+1))
	b.Write(tmp, b.BigEndian, int32(4))
	tmp.WriteString("PONG")
	tmp.WriteByte(CMD_PING)
	PONG_BYTES = tmp.Bytes()
}

type MoaClientCodeC struct {
	protocol.RedisGetCodec
}

//直接获取data
func (self MoaClientCodeC) Read(reader *bufio.Reader) (*bytes.Buffer, error) {

	line, isPrefix, err := reader.ReadLine()
	if nil != err {
		return nil, errors.New("MoaClientCodeC Read Command Args Count Packet Err " + err.Error())
	}
	start := bytes.HasPrefix(line, DOLLAR)
	if start {
		//$n\r\n[data]\r\n
		//获取到共有多少个\r\n
		l, _ := strconv.ParseInt(string(line[1:]), 10, 64)
		length := int(l)
		if length >= self.MaxFrameLength {
			return nil, errors.New(fmt.Sprintf("MoaClientCodeC Too Large Packet %d/%d", length, self.MaxFrameLength))
		}
		//读取数组长度和对应的值
		//bodyLen+body+CommandType
		//4B+body+1B 是为了给将长度协议类型附加在dataBuff的末尾
		tmp := bytes.NewBuffer(make([]byte, 0, 4+length+1))
		b.Write(tmp, b.BigEndian, int32(length))
		dl := 0
		for {
			line, isPrefix, err = reader.ReadLine()
			if nil != err {
				return nil, errors.New("MoaClientCodeC Read Packet Data Err " + err.Error())
			}

			//如果超过了给定的长度则忽略
			if (dl + len(line)) > length {
				return nil, errors.New(fmt.Sprintf("MoaClientCodeC Invalid Packet Data %d/%d ", (dl + len(line)), length))
			}

			//没有读取完这个命令的字节继续读取
			l, er := tmp.Write(line)
			if nil != err {
				return nil, errors.New("MoaClientCodeC Write Packet Into Buff  Err " + er.Error())
			}
			//读取完这个命令的字节
			if !isPrefix {
				break
			} else {

			}
			dl += l
		}

		//写入命令类型
		tmp.WriteByte(CMD_GET)
		//得到了get和数据将数据返回出去
		return tmp, nil
	} else if bytes.HasPrefix(line, PONG) {
		//返回的是字符串、看看是不是PONG
		return bytes.NewBuffer(PONG_BYTES), nil
	} else {
		return nil, errors.New("MoaClientCodeC Error Packet Prototol Is Not Get Response " + string(line[:0]))
	}
}

//反序列化
//包装为packet，但是头部没有信息
func (self MoaClientCodeC) UnmarshalPacket(buff *bytes.Buffer) (*packet.Packet, error) {
	var l int32
	b.Read(buff, b.BigEndian, &l)
	d := buff.Bytes()
	return packet.NewRespPacket(0, d[l], d[:l]), nil
}

//序列化
//直接获取data
// 	GET :*2\r\n$3\r\nGET\r\n$n\r\n[data]\r\n
//	PING:*1\r\n$4\r\nPING\r\n
func (self MoaClientCodeC) MarshalPacket(p *packet.Packet) []byte {

	switch p.Header.CmdType {
	case CMD_GET:
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
		fmt.Println(string(b))
		return b
	case CMD_PING:
		return p.Data
	default:
		panic(fmt.Sprintf("UnSupport Redis Protocol %d", p.Header.CmdType))
	}
}
