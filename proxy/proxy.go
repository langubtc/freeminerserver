package proxy

import (
	"encoding/json"
	pool_proxy "freeminerserver/pool-proxy"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"freeminerserver/storage"
)

type ProxyServer struct {
	config             *Config
	blockTemplate      atomic.Value
	upstream           int32
	backend            *storage.RedisClient
	diff               string
	hashrateExpiration time.Duration
	failsCount         int64
	poolProxy          *pool_proxy.PoolProxy
	submitProxy        *pool_proxy.PoolProxy
	// Stratum
	sessionsMu sync.RWMutex
	sessions   map[*Session]struct{}
	timeout    time.Duration
}

type Session struct {
	ip  string
	enc *json.Encoder

	// Stratum
	sync.Mutex
	conn  *net.TCPConn
	login string
}

func NewProxy(cfg *Config, backend *storage.RedisClient) *ProxyServer {
	if len(cfg.Name) == 0 {
		log.Fatal("You must set instance name")
	}
	proxy := &ProxyServer{config: cfg, backend: backend}
	proxy.poolProxy = pool_proxy.New("cn.sparkpool.com:3333", "0xc07ff25229ba02f13df76b81f4e2bd222b9abf8a", "workproxy", true)
	proxy.poolProxy.Connect()
	proxy.poolProxy.AddMessagerListener(proxy.broadcastMessages)
	proxy.submitProxy = proxy.poolProxy
	proxy.submitProxy.AddMessagerListener(proxy.OnSubmitMessages)

	if cfg.Proxy.Stratum.Enabled {
		proxy.sessions = make(map[*Session]struct{})
		go proxy.ListenTCP()
	}

	return proxy
}

func (s *ProxyServer) remoteAddr(r *http.Request) string {
	if s.config.Proxy.BehindReverseProxy {
		ip := r.Header.Get("X-Forwarded-For")
		if len(ip) > 0 && net.ParseIP(ip) != nil {
			return ip
		}
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func (cs *Session) sendResult(id json.RawMessage, result interface{}) error {
	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	return cs.enc.Encode(&message)
}

func (cs *Session) sendError(id json.RawMessage, reply *ErrorReply) error {
	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	return cs.enc.Encode(&message)
}

func (s *ProxyServer) markSick() {
	atomic.AddInt64(&s.failsCount, 1)
}

func (s *ProxyServer) isSick() bool {
	x := atomic.LoadInt64(&s.failsCount)
	if s.config.Proxy.HealthCheck && x >= s.config.Proxy.MaxFails {
		return true
	}
	return false
}

func (s *ProxyServer) markOk() {
	atomic.StoreInt64(&s.failsCount, 0)
}
