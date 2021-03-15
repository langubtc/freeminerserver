package proxy

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/panglove/freeminerserver/util"
)

const (
	MaxReqSize = 1024
)
type RespParamInfo struct {
	Result []string `json:"result"`
}
func (s *ProxyServer) ListenTCP() {
	timeout := util.MustParseDuration(s.config.Proxy.Stratum.Timeout)
	s.timeout = timeout

	addr, err := net.ResolveTCPAddr("tcp", s.config.Proxy.Stratum.Listen)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	server, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer server.Close()

	log.Printf("Stratum listening on %s", s.config.Proxy.Stratum.Listen)
	var accept = make(chan int, s.config.Proxy.Stratum.MaxConn)
	n := 0

	for {
		conn, err := server.AcceptTCP()
		if err != nil {
			continue
		}
		conn.SetKeepAlive(true)

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

		n += 1
		cs := &Session{conn: conn, ip: ip}

		accept <- n
		go func(cs *Session) {
			err = s.handleTCPClient(cs)
			if err != nil {
				s.removeSession(cs)
				conn.Close()
			}
			<-accept
		}(cs)
	}
}

func (s *ProxyServer) handleTCPClient(cs *Session) error {
	cs.enc = json.NewEncoder(cs.conn)
	connbuff := bufio.NewReaderSize(cs.conn, MaxReqSize)
	s.setDeadline(cs.conn)

	for {
		data, isPrefix, err := connbuff.ReadLine()
		if isPrefix {
			log.Printf("Socket flood detected from %s", cs.ip)
			return err
		} else if err == io.EOF {
			log.Printf("Client %s disconnected", cs.ip)
			s.removeSession(cs)
			break
		} else if err != nil {
			log.Printf("Error reading from socket: %v", err)
			return err
		}

		if len(data) > 1 {
			var req StratumReq
			log.Println("client",cs.ip," receive:",string(data))

			err = json.Unmarshal(data, &req)
			if err != nil {
				log.Printf("Malformed stratum request from %s: %v", cs.ip, err)
				return err
			}
			s.setDeadline(cs.conn)
			err = cs.handleTCPMessage(s,&req,data)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (cs *Session) handleTCPMessage(s *ProxyServer, req *StratumReq,data []byte) error {
	// Handle RPC methods
	switch req.Method {
	case "eth_getWork":
		return nil
	case "eth_submitHashrate":
		newStr :=string(data)
		newStr = strings.ReplaceAll(newStr,"\"worker\":\""+""+req.Worker+"\"","\"worker\":"+"\""+s.submitProxy.Name+"\"")
		data = []byte(newStr)
		log.Println("提交算力:",newStr)

		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Println("Malformed stratum request params from", cs.ip)
			return err
		}
		s.backend.WriteShareHashRate(req.Worker,int(util.HexToInt(params[0]).Int64()))

		return cs.sendTCPResult(req.Id, true)
	case "eth_submitLogin":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Println("Malformed stratum request params from", cs.ip)
			return err
		}
		log.Println("statum eth_submitLogin", params)
		reply, errReply := s.handleLoginRPC(cs, params, req.Worker)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		log.Println("statum eth_submitLogin", reply, errReply)
		return cs.sendTCPResult(req.Id, reply)
	case "eth_submitWork":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			log.Println("Malformed stratum request params from", cs.ip)
			return err
		}
		_, errReply := s.handleTCPSubmitRPC(cs, req.Worker, params,req.Id)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return nil
	default:
		//过滤其余无用消息
		//s.poolProxy.SendMessageData(data)
	}
	return nil
}

func (cs *Session) sendTCPResult(id json.RawMessage, result interface{}) error {
	cs.Lock()
	defer cs.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	return cs.enc.Encode(&message)
}

func (cs *Session) pushNewJob(result interface{}) error {
	cs.Lock()
	defer cs.Unlock()
	// FIXME: Temporarily add ID for Claymore compliance
	message := JSONPushMessage{Version: "2.0", Result: result, Id: 0}
	return cs.enc.Encode(&message)
}
func (cs *Session) SendMessage(result string) error {
	cs.Lock()
	defer cs.Unlock()
	// FIXME: Temporarily add ID for Claymore compliance
	newInfo :=new(interface{})
	json.Unmarshal([]byte(result),newInfo)
	return cs.enc.Encode(newInfo)
}

func (cs *Session) sendTCPError(id json.RawMessage, reply *ErrorReply) error {
	cs.Lock()
	defer cs.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	err := cs.enc.Encode(&message)
	if err != nil {
		return err
	}
	return errors.New(reply.Message)
}

func (self *ProxyServer) setDeadline(conn *net.TCPConn) {
	conn.SetDeadline(time.Now().Add(self.timeout))
}

func (s *ProxyServer) registerSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.sessions[cs] = struct{}{}
}

func (s *ProxyServer) removeSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	delete(s.sessions, cs)
}
func (s *ProxyServer) broadcastMessages(msg string) {

	newParams :=new(RespParamInfo)
	err :=json.Unmarshal([]byte(msg),newParams)
	if err!=nil || len(newParams.Result)<2{
		return
	}

	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	count := len(s.sessions)
	log.Printf("Broadcasting new msg to %v stratum miners", count)

	start := time.Now()
	bcast := make(chan int, 1024)
	n := 0

	for m, _ := range s.sessions {
		n++
		bcast <- n

		go func(cs *Session) {

			err := cs.SendMessage(msg)
			<-bcast
			if err != nil {
				log.Printf("board msg error to %v@%v: %v", cs.login, cs.ip, err)
				s.removeSession(cs)
			} else {
				s.setDeadline(cs.conn)
			}
		}(m)
	}
	log.Printf("board msg  finished %s", time.Since(start))
}