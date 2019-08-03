package main

import (
	"container/list"
	"crypto/rc4"
	"gate/tools"
	"log"
	"net"
	"sync"
	"sync/atomic"

	//"sync/atomic"
	"time"
)

const (
	SESSION_KEYEXCG    = 0x1 // 是否已经交换完毕KEY
	SESSION_ENCRYPT    = 0x2 // 是否可以开始加密
	SESSION_KICKED_OUT = 0x4 // 踢掉
	SESSION_AUTHORIZED = 0x8 // 已授权访问
)

var GateSessionPool *SsPool //池

type SsPool struct {
	//全部session缓存池
	wholePool map[int]*Session
	//空闲的session池
	freePool *list.List
	//工作的session池
	workPool *list.List
	//锁
	lock *sync.Mutex
}

type Session struct {
	Id     int
	Gid    int           // 游戏服ID;e.g.: game1,game2
	UserId int64         // 玩家ID
	e      *list.Element //链表指针
	IP     net.IP
	Conn   net.Conn

	CoMsg   chan []byte //从conn中接受到的信息
	ReMsg   chan []byte // 返回给客户端的异步消息
	GaMsg   chan []byte //转发送给Game服务器的消息
	Encoder *rc4.Cipher // 加密器
	Decoder *rc4.Cipher // 解密器
	Die     chan bool   // 会话关闭信号

	IsActive int32 // 状态标记在free池为0，在work池为1，如果session不再活动，则标记为0，等待回收

	// 时间相关
	ConnectTime    time.Time // TCP链接建立时间
	PacketTime     time.Time // 当前包的到达时间
	LastPacketTime time.Time // 前一个包到达时间

	PacketCount     int // 对收到的包进行计数，避免恶意发包
	PacketCount1Min int // 每分钟的包统计，用于RPM判断
}

//从缓存池获取一个session
func (self *SsPool) GetS(ip net.IP, conn net.Conn) (_s *Session) {
	self.lock.Lock()
	defer self.lock.Unlock()

	if self.freePool.Len() == 0 {
		return
	}
	_e := self.freePool.Front()
	_s = _e.Value.(*Session)
	self.freePool.Remove(_e)
	self.workPool.PushBack(_s)
	_s.IP = ip
	_s.Conn = conn
	_s.IsActive = true
	return
}

//定时从work池中清理已经过期session回free池
func (self *SsPool) CleanWP() {
	self.lock.Lock()
	defer self.lock.Unlock()

	var _s *Session
	var _next *list.Element
	for _e := self.workPool.Front(); _e != nil; _e = _next {
		_next = _e.Next()
		_s = _e.Value.(*Session)
		if atomic.LoadInt32(&_s.IsActive) == 0 {
			self.workPool.Remove(_e)
			self.freePool.PushBack(_s)
			_s.Reset()
		}
	}
}

//Session方法
func sessionNew() Session {
	return Session{
		ReMsg: make(chan []byte, 8),
		GaMsg: make(chan []byte),
		Die:   make(chan bool),
	}
}

//初始化session
func (self *Session) Init(ip net.IP, conn net.Conn) {
	self.IP = ip
	self.Conn = conn
	atomic.StoreInt32(&self.IsActive, 1)
}

//重置session
func (self *Session) Reset() {
	self.IP = nil
	self.Conn = nil
	self.Gid = 0
	self.UserId = 0
	self.Encoder = nil
	self.Decoder = nil
	self.PacketCount = 0
	self.PacketCount1Min = 0

	self.IsActive = false
}

//启动session,注意每个session全局只会启动一次
func (self *Session) Start() {
	defer tools.PrintPanicStack()

	var _cm, _rm, _gm []byte
	var _ok bool
	var err error
	//创建加密解密
	self.Encoder, err = rc4.NewCipher([]byte(tools.CONST_SOCKET_CRYPTO))
	if err != nil {
		log.Panic("RC4 Crypto is wrong:", err)
	}
	self.Decoder, err = rc4.NewCipher([]byte(tools.CONST_SOCKET_CRYPTO))
	if err != nil {
		log.Panic("RC4 Crypto is wrong:", err)
	}
	for {
		select {
		//从conn中接收消息
		case _cm, _ok = <-self.CoMsg:
			if !_ok {

			}
		//从gate中接收返回消息
		case _rm = <-self.ReMsg:
		//接受gameserver消息
		case _gm = <-self.GaMsg:

		}
	}
}

func init() {
	//初始化sessionpool
	GateSessionPool = &SsPool{
		wholePool: make(map[int]*Session),
		freePool:  new(list.List),
		workPool:  new(list.List),
		lock:      new(sync.Mutex),
	}
	for i := 1; i <= tools.CONST_GATE_SSPOOL_MAXLEN; i++ {
		_ss := sessionNew()
		GateSessionPool.wholePool[GateId*tools.CONST_GATE_SSPOOL_MAXLEN+i-1] = &_ss
		GateSessionPool.freePool.PushBack(&_ss)
		_ss.e = GateSessionPool.freePool.Back()
	}
	_t := time.NewTicker(time.Minute)
	//启动定时清理服务
	for {
		<-_t.C
		GateSessionPool.CleanWP()
	}
}
