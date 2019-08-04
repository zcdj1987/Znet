package tools

import "time"

const (
	//GATE相关
	CONST_SOCKET_BUFLEN      = 32767
	CONST_SOCKET_DEADLINE    = 60 * time.Second
	CONST_SOCKET_CRYPTO      = "fea534584cbd0b49390ec8b6f8b681e8"
	CONST_GATE_SSPOOL_MAXLEN = 1000
)

const (
	//RPC相关
	CONST_RPC_G2G_TYPE_REG    = iota //新用户注册
	CONST_RPC_G2G_TYPE_NORMAL        //正常交互消息
	CONST_RPC_G2G_TYPE_QUIT          //退出的消息
)
