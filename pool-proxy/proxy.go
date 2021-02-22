package pool_proxy

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type JSONRpcResp struct {
	Id      int         `json:"id"`
	Version string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   interface{} `json:"error,omitempty"`
}
type PoolProxy struct {
	Url                 string
	Address             string
	Name                string
	IsReadMsg           bool
	ReadMux sync.RWMutex
	isReconnecting bool
	MessageListenerList []func(string)
	net.Conn
}
type JSONRpcReq struct {
	Id     int      `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}

type StratumReq struct {
	JSONRpcReq
	Worker string `json:"worker"`
}

func New(url string, address string, name string, isRead bool) *PoolProxy {
	newPoolProxy := new(PoolProxy)
	newPoolProxy.Url = url
	newPoolProxy.Address = address
	newPoolProxy.Name = name
	newPoolProxy.IsReadMsg = isRead
	return newPoolProxy
}
func (this *PoolProxy) Connect() {
	go func() {
		this.ReadMux.Lock()
		this.isReconnecting = true

		this.ReadMux.Unlock()
		newCon, err := net.Dial("tcp", this.Url)
		if err != nil {
			log.Println("Connect Pool Error", err, "Wait 5 Second Reconnect...")
			time.Sleep(time.Second * 5)
			this.Connect()
		} else {
			log.Println("Connnected proxy server:", this.Url)
			this.Conn = newCon
			this.Login()

			this.ReadMux.Lock()
			this.isReconnecting = false
			this.ReadMux.Unlock()
			if this.IsReadMsg {
				this.ReadMessage()
			}
		}
	}()
}

func (this *PoolProxy) Login() {
	this.SendMessage(1, "eth_submitLogin", []string{this.Address}, this.Name)
}
func (this *PoolProxy) AddMessagerListener(fc func(string)) {
	this.MessageListenerList = append(this.MessageListenerList, fc)
}
func (this *PoolProxy) SendMessage(id int, method string, params []string, work string) {
	if this.Conn == nil {
		return
	}
	newMsg := new(StratumReq)
	newMsg.Id = id
	newMsg.Method = method
	newMsg.Params = params
	newMsg.Worker = work

	newMsgBuff, _ := json.Marshal(newMsg)

	log.Println("send message :", string(newMsgBuff))
	_, err := this.Write([]byte(string(newMsgBuff) + "\n"))

	if err != nil {
		log.Println("Send Message Error :", err)
	}

}
func (this *PoolProxy) SendMessageData(data []byte) {
	if this.Conn == nil {
		return
	}
	log.Println("send message :", string(data))
	_, err := this.Write([]byte(string(data) + "\n"))

	if err != nil {
		log.Println("Send Message Error :", err)
	}

}
func (this *PoolProxy) SendMessageSync(id int, method string, params []string, work string)string {
	if this.Conn == nil {
		return ""
	}
	newMsg := new(StratumReq)
	newMsg.Id = id
	newMsg.Method = method
	newMsg.Params = params
	newMsg.Worker = work

	newMsgBuff, _ := json.Marshal(newMsg)

	log.Println("send message :", string(newMsgBuff))
	_, err := this.Write([]byte(string(newMsgBuff) + "\n"))

	if err != nil {
		log.Println("Send Message Error :", err)
		return ""
	}

	lineReader := bufio.NewReader(this)

	buff, isPrefix, err := lineReader.ReadLine()

	if isPrefix {
		log.Printf("Socket flood detected")
		return ""
	} else if err == io.EOF {
		log.Printf("Socket %s disconnected", err)

		this.ReadMux.Lock()
		if !this.isReconnecting {
			this.ReadMux.Unlock()
			this.Connect()
		}
		return ""
	}
	if err != nil {
		log.Println("read message error", err)
		return ""
	}

	msgStr := string(buff)

	log.Println("receive message:", msgStr)

	return msgStr

}
func (this *PoolProxy) SendMessageDataSync(data []byte) string {
	if this.Conn == nil {
		return ""
	}
	log.Println("send message :", string(data))
	_, err := this.Write([]byte(string(data) + "\n"))

	if err != nil {
		log.Println("Send Message Error :", err)
		return ""
	}

	lineReader := bufio.NewReader(this)

	buff, isPrefix, err := lineReader.ReadLine()

	if isPrefix {
		log.Printf("Socket flood detected")
		return ""
	} else if err == io.EOF {
		log.Printf("Socket %s disconnected", err)

		this.ReadMux.Lock()
		if !this.isReconnecting {
			this.ReadMux.Unlock()
			this.Connect()
		}
		return ""
	}
	if err != nil {
		log.Println("read message error", err)
		return ""
	}

	msgStr := string(buff)

	log.Println("receive message:", msgStr)

	return msgStr
}
func (this *PoolProxy) ReadMessage() {
	go func() {
		lineReader := bufio.NewReader(this)
		for {

			buff, isPrefix, err := lineReader.ReadLine()
			if isPrefix {
				log.Printf("Socket flood detected")
				continue
			} else if err == io.EOF {
				log.Printf("Socket %s disconnected", err)
				break
			}
			if err != nil {
				log.Println("read message error", err)
				continue
			}

			msgStr := string(buff)

			for kc, _ := range this.MessageListenerList {

				this.MessageListenerList[kc](msgStr)

			}


		}
		this.Connect()
	}()
}
