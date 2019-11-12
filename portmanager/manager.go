package portmanager

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"sync"

	"github.com/jacobsa/go-serial/serial"
)

type Port struct {
	Device string
	Socket io.ReadWriteCloser
	cancel context.CancelFunc
	ctx    context.Context
}

func (p *Port) Context() context.Context {
	return p.ctx
}

func (p *Port) Close() {
	p.cancel()
}

type PortManager struct {
	ports map[string]*Port
	rm    sync.RWMutex
}

func New() *PortManager {
	return &PortManager{
		ports: map[string]*Port{},
	}
}

func (pm *PortManager) Connect(dev string, baud uint) (*Port, error) {
	pm.rm.Lock()
	defer pm.rm.Unlock()
	c := serial.OpenOptions{
		PortName:        fmt.Sprintf("/dev/%s", dev),
		BaudRate:        baud,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}
	s, err := serial.Open(c)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Port{
		Device: dev,
		Socket: s,
		ctx:    ctx,
		cancel: cancel,
	}
	pm.ports[dev] = p
	return p, nil
}

func (pm *PortManager) Disconnect(dev string) {
	pm.rm.Lock()
	defer pm.rm.Unlock()
	log.Print(dev)
	p := pm.ports[dev]
	p.cancel()
	p.Socket.Close()
	delete(pm.ports, dev)
}

func (pm *PortManager) Port(dev string) *Port {
	pm.rm.RLock()
	defer pm.rm.RUnlock()
	return pm.ports[dev]
}

func (pm *PortManager) Ports() []string {
	pm.rm.RLock()
	defer pm.rm.RUnlock()
	out := make([]string, len(pm.ports))
	i := 0
	for k := range pm.ports {
		out[i] = k
		i++
	}
	sort.Slice(out, func(a, b int) bool { return out[b] > out[a] })
	return out
}
