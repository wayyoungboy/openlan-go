package point

import (
	"context"
	"crypto/tls"
	"github.com/lightstar-dev/openlan-go/point/models"
	"net"

	"github.com/lightstar-dev/openlan-go/libol"
	"github.com/songgao/water"
)

type Point struct {
	BrName string
	IfAddr string

	tcpWorker *TcpWorker
	tapWorker *TapWorker
	brIp      net.IP
	brNet     *net.IPNet
	config    *models.Config
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewPoint(config *models.Config) (p *Point) {
	var tlsConf *tls.Config
	if config.Tls {
		tlsConf = &tls.Config{InsecureSkipVerify: true}
	}
	client := libol.NewTcpClient(config.Addr, tlsConf)
	p = &Point{
		BrName:    config.BrName,
		IfAddr:    config.IfAddr,
		tcpWorker: NewTcpWorker(client, config),
		config:    config,
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.newDevice()
	return
}

func (p *Point) newDevice() {
	dev, err := water.New(water.Config{DeviceType: water.TUN})
	if err != nil {
		libol.Fatal("NewPoint: %s", err)
		return
	}

	libol.Info("NewPoint.device %s", dev.Name())
	p.tapWorker = NewTapWorker(dev, p.config)
}

func (p *Point) UpLink() error {
	if p.GetDevice() == nil {
		p.newDevice()
	}
	if p.GetDevice() == nil {
		return libol.Errer("create device.")
	}
	return nil
}

func (p *Point) Start() {
	libol.Debug("Point.Start Darwin.")

	p.UpLink()
	if err := p.tcpWorker.Connect(); err != nil {
		libol.Error("Point.Start %s", err)
	}

	go p.tapWorker.GoRecv(p.ctx, p.tcpWorker.DoSend)
	go p.tapWorker.GoLoop(p.ctx)

	go p.tcpWorker.GoRecv(p.ctx, p.tapWorker.DoSend)
	go p.tcpWorker.GoLoop(p.ctx)
}

func (p *Point) Stop() {
	defer libol.Catch("Point.Stop")

	p.cancel()
	p.tapWorker.Stop()
	p.tcpWorker.Stop()
}

func (p *Point) GetClient() *libol.TcpClient {
	if p.tcpWorker != nil {
		return p.tcpWorker.Client
	}
	return nil
}

func (p *Point) GetDevice() *water.Interface {
	if p.tapWorker != nil {
		return p.tapWorker.Dev
	}
	return nil
}

func (p *Point) UpTime() int64 {
	client := p.GetClient()
	if client != nil {
		return client.UpTime()
	}
	return 0
}

func (p *Point) State() string {
	client := p.GetClient()
	if client != nil {
		return client.GetState()
	}
	return ""
}

func (p *Point) Addr() string {
	client := p.GetClient()
	if client != nil {
		return client.Addr
	}
	return ""
}

func (p *Point) IfName() string {
	dev := p.GetDevice()
	if dev != nil {
		return dev.Name()
	}
	return ""
}
