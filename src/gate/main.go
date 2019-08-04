package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"gate/tools"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"os"
	"runtime"
	"time"
)

var h bool
var Server_King string
var Version string
var port string
var GateId int

func main() {
	defer tools.PrintPanicStack()
	//所有gate都是单进程服务器
	runtime.GOMAXPROCS(1)
	//启动初始化
	flag.Parse()
	if h {
		flag.Usage()
	}
	log.SetFormatter(&log.JSONFormatter{}) //设定log输出格式

	go tcpServer() //启动tcp服务

	select {}
}

func tcpServer() {
	tcpAddr, err := net.ResolveTCPAddr("tpc4", port)
	tools.CheckErrorExit(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	tools.CheckErrorExit(err)

	log.Info("Server Start,listen on:", listener.Addr())

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Warn("accept failed:", err)
			continue
		}
		conn.SetReadBuffer(tools.CONST_SOCKET_BUFLEN)
		conn.SetWriteBuffer(tools.CONST_SOCKET_BUFLEN)

		go handleClient(conn)
	}
}

func handleClient(conn net.Conn) {
	defer tools.PrintPanicStack()
	defer conn.Close()

	var sess *Session

	_head := make([]byte, 2) //包长度
	host, port, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		log.Error("cannot get remote address:", err)
		return
	}
	ip := net.ParseIP(host)
	log.Infof("new connection from:%v port:%v", host, port)

	//从session池中获取一个缓存
	sess = GateSessionPool.GetS(ip, conn)

	//读取消息
	for {
		//设置读取超时，默认1分钟
		conn.SetReadDeadline(time.Now().Add(tools.CONST_SOCKET_DEADLINE))

		//读取包长
		n, err := io.ReadFull(conn, _head)
		if err != nil {
			log.Warningf("read header failed, ip:%v reason:%v size:%v", sess.IP, err, n)
			return
		}
		//大头对齐
		size := binary.BigEndian.Uint16(_head)

		// alloc a byte slice of the size defined in the header for reading data
		payload := make([]byte, size)
		n, err = io.ReadFull(conn, payload)
		if err != nil {
			log.Warningf("read payload failed, ip:%v reason:%v size:%v", sess.IP, err, n)
			return
		}

		// deliver the data to the input queue of agent()
		select {
		case in <- payload: // payload queued
		case <-sess.Die:
			log.Warningf("connection closed by logic, flag:%v ip:%v", sess.Flag, sess.IP)
			return
		}
	}
}

func init() {
	flag.BoolVar(&h, "h", false, "show server help")
	flag.StringVar(&Server_King, "serK", "127.0.0.1:1987", "server king address")
	flag.StringVar(&Version, "v", "1.0.0", "server version")
	flag.StringVar(&port, "l", ":6600", "external listening port")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `nginx version: nginx/1.10.0
		Usage: nginx [-hvVtTq] [-s signal] [-c filename] [-p prefix] [-g directives]
		
		Options:
		`)
		flag.PrintDefaults()
	}
}
