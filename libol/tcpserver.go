package libol

import (
	"crypto/tls"
	"net"
)

type OnTcpServer interface {
	OnClient(*TcpClient) error
	OnRecv(*TcpClient, []byte) error
	OnClose(*TcpClient) error
}

type TcpServer struct {
	Addr     string
	RxCount  int64
	TxCount  int64
	DrpCount int64
	AcpCount int64
	ClsCount int64
	TlsConf  *tls.Config

	listener   net.Listener
	maxClient  int
	clients    map[*TcpClient]bool
	onClients  chan *TcpClient
	offClients chan *TcpClient
}

func NewTcpServer(listen string, config *tls.Config) (t *TcpServer) {
	t = &TcpServer{
		Addr:       listen,
		listener:   nil,
		maxClient:  1024,
		clients:    make(map[*TcpClient]bool, 1024),
		onClients:  make(chan *TcpClient, 4),
		offClients: make(chan *TcpClient, 8),
		TlsConf:    config,
	}

	if err := t.Listen(); err != nil {
		Debug("NewTcpServer %s", err)
	}

	return
}

func (t *TcpServer) Listen() (err error) {
	Info("TcpServer.Start %s,%p", t.Addr, t.TlsConf)

	if t.TlsConf != nil {
		t.listener, err = tls.Listen("tcp", t.Addr, t.TlsConf)
		if err != nil {
			Info("TcpServer.Listen: %s", err)
			return
		}
	} else {
		t.listener, err = net.Listen("tcp", t.Addr)
		if err != nil {
			Info("TcpServer.Listen: %s", err)
			t.listener = nil
			return
		}
	}

	return nil
}

func (t *TcpServer) Close() {
	if t.listener != nil {
		t.listener.Close()
		Info("TcpServer.Close: %s", t.Addr)
		t.listener = nil
	}
}

func (t *TcpServer) GoAccept() {
	Debug("TcpServer.GoAccept")
	if t.listener == nil {
		Error("TcpServer.GoAccept: invalid listener")
	}

	defer t.Close()
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			Error("TcpServer.GoAccept: %s", err)
			return
		}

		t.AcpCount++
		t.onClients <- NewTcpClientFromConn(conn)
	}

	return
}

func (t *TcpServer) GoLoop(on OnTcpServer) {
	Debug("TcpServer.GoLoop")
	defer t.Close()
	for {
		select {
		case client := <-t.onClients:
			Debug("TcpServer.addClient %s", client.Addr)
			if on.OnClient != nil {
				on.OnClient(client)
			}
			t.clients[client] = true
			go t.GoRecv(client, on.OnRecv)
		case client := <-t.offClients:
			if ok := t.clients[client]; ok {
				Debug("TcpServer.delClient %s", client.Addr)
				t.ClsCount++
				if on.OnClose != nil {
					on.OnClose(client)
				}
				client.Close()
				delete(t.clients, client)
			}
		}
	}
}

func (t *TcpServer) GoRecv(client *TcpClient, onRecv func(*TcpClient, []byte) error) {
	Debug("TcpServer.GoRecv: %s", client.Addr)
	for {
		data := make([]byte, 4096)
		length, err := client.RecvMsg(data)
		if err != nil {
			t.offClients <- client
			break
		}

		if length > 0 {
			t.RxCount++
			Debug("TcpServer.GoRecv: length: %d ", length)
			Debug("TcpServer.GoRecv: data  : % x", data[:length])
			onRecv(client, data[:length])
		}
	}
}
