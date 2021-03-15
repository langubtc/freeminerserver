package proxy

import (
	"encoding/json"
	"github.com/ethereum/ethash"
	"github.com/panglove/freeminerserver/rpc"
	"log"
	"sync"
)

type SubmitInfo struct {
	cs     *Session
	login  string
	id     string
	ip     string
	reqId  []byte
	params []string
}

const miniSubmitId = 1000

var submitId = miniSubmitId
var submitMux sync.RWMutex
var hasher = ethash.New()

var submitMap = make(map[int]*SubmitInfo)

func (s *ProxyServer) processShare(cs *Session, login, id, ip string, params []string, reqId []byte) {
	submitMux.Lock()
	submitId++
	if submitId > 100000000000 {
		submitId = miniSubmitId
	}
	submitMap[submitId] = &SubmitInfo{cs: cs, login: login, id: id, ip: ip, params: params, reqId: reqId}
	submitMux.Unlock()
	s.submitProxy.SendMessage(submitId, "eth_submitWork", params, s.submitProxy.Name)
}
func (s *ProxyServer) OnSubmitMessages(result string) {

	if len(result) == 0 {
		return
	}
	newSubResult := new(rpc.JSONRpcResp)
	err := json.Unmarshal([]byte(result), newSubResult)

	if err != nil || newSubResult.Id == nil || newSubResult.Result == nil {
		return
	}
	var idIndex int
	json.Unmarshal(*newSubResult.Id, &idIndex)
	if idIndex <= miniSubmitId || submitMap[idIndex] == nil {
		return
	}
	subInfo := *submitMap[idIndex]

	log.Println("submit reback:", result, subInfo)

	var reply bool
	err = json.Unmarshal(*newSubResult.Result, &reply)
	if err != nil {
		log.Printf("share submission failure  %s for %s: %s", subInfo.ip, subInfo.id, subInfo.params)
		subInfo.cs.sendTCPError(subInfo.reqId, &ErrorReply{Code: 23, Message: "Invalid share"})
		return

	} else if !reply {
		log.Printf("share rejected %s for %s: %s", subInfo.ip, subInfo.id, subInfo.params)
		subInfo.cs.sendTCPError(subInfo.reqId, &ErrorReply{Code: 23, Message: "Invalid share"})
		return
	} else {
		exist, err := s.backend.WriteShare(subInfo.id, subInfo.params)
		if exist {
			subInfo.cs.sendTCPError(subInfo.reqId, &ErrorReply{Code: 22, Message: "Duplicate share"})
			return
		}
		if err != nil {
			log.Println("Failed to insert share data into backend:", err)
		}

	}
	s.backend.WriteShareTime(subInfo.id)
	log.Printf("share success  %s for %s: %s", subInfo.ip, subInfo.id, subInfo.params)
	subInfo.cs.sendTCPResult(subInfo.reqId, &reply)
}
