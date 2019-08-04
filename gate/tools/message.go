package tools

//gate服务器和game服务器之间通信的RPC消息
type G2GMsg struct {
	GateId  int
	UserIds []int64
	Typ     int8 //消息的类型
	Msg     []byte
}

func NewG2GMsg() *G2GMsg {
	return &G2GMsg{
		GateId:  0,
		UserIds: nil,
		Typ:     0,
		Msg:     nil,
	}
}
