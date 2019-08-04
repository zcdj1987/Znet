package main

import (
	"container/list"
	"crypto/rc4"
	"gate/tools"
	log "github.com/sirupsen/logrus"
	"net"
	//"sync/atomic"
	"time"
)

var GateSessionPool *SsPool //池

type SsPool struct {
	//全部session缓存池
	wholePool map[int]*Session
	//空闲的session池
	freePool *list.List
	//工作的session池
	workPool *list.List
}

type Session struct {
	IsActive bool // 如果false，则不再收发消息，等待被回收
	IP       net.IP
	Conn     net.Conn
	Die      chan bool // 会话关闭信号

	id    int
	useId int64         // 玩家ID
	e     *list.Element //链表指针

	//session相关消息
	coMsg   chan []byte         //从conn中接受到的信息
	reMsg   chan []tools.RpcMsg // 返回给客户端的异步消息
	sysMsg  chan []byte         //转发送给Game服务器的消息
	encoder *rc4.Cipher         // 加密器
	decoder *rc4.Cipher         // 解密器

	// 时间相关
	ConnectTime    time.Time // TCP链接建立时间
	PacketTime     time.Time // 当前包的到达时间
	LastPacketTime time.Time // 前一个包到达时间

	PacketCount     int // 对收到的包进行计数，避免恶意发包
	PacketCount1Min int // 每分钟的包统计，用于RPM判断

	only bool //唯一启动标记，表示start方法只能启动一次
}

//从缓存池获取一个session
func (self *SsPool) GetS(ip net.IP, conn net.Conn) (_s *Session) {
	if self.freePool.Len() == 0 {
		log.Warning("gate server is full:", GateId)
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
	var _s *Session
	var _next *list.Element
	for _e := self.workPool.Front(); _e != nil; _e = _next {
		_next = _e.Next()
		_s = _e.Value.(*Session)
		if _s.IsActive {
			self.workPool.Remove(_e)
			self.freePool.PushBack(_s)
			_s.Reset()
		}
	}
}

//Session方法
func sessionNew() Session {
	return Session{
		coMsg:  make(chan []byte, 8),
		reMsg:  make(chan []byte, 8),
		sysMsg: make(chan []byte),
		Die:    make(chan bool),
	}
}

//重置session
func (self *Session) Reset() {
	self.IP = nil
	self.Conn = nil
	self.useId = 0
	self.encoder = nil
	self.decoder = nil
	self.PacketCount = 0
	self.PacketCount1Min = 0
	self.IsActive = false
}

//启动session,注意每个session全局只会启动一次
func (self *Session) Start() {
	defer tools.PrintPanicStack()

	if self.only {
		return
	}
	self.only = true

	var _cm, _rm, _gm []byte
	var _ok bool
	var err error
	//创建加密解密
	self.encoder, err = rc4.NewCipher([]byte(tools.CONST_SOCKET_CRYPTO))
	if err != nil {
		log.Panic("RC4 Crypto is wrong:", err)
	}
	self.decoder, err = rc4.NewCipher([]byte(tools.CONST_SOCKET_CRYPTO))
	if err != nil {
		log.Panic("RC4 Crypto is wrong:", err)
	}
	for {
		select {
		//从conn中接收消息
		case _cm = <-self.coMsg:
			if self.IsActive {
				//处理将消息封装之后，通过rpc发往game服
			}
		//从gate中接收返回消息
		case _rm = <-self.reMsg:
		//接受gameserver消息
		case _gm = <-self.sysMsg:
		case <-self.Die:
			self.Reset()
		}
	}
}
func (self *Session) SignDie() {
	self.Die <- true
}

func init() {
	//初始化sessionpool
	GateSessionPool = &SsPool{
		wholePool: make(map[int]*Session),
		freePool:  new(list.List),
		workPool:  new(list.List),
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
