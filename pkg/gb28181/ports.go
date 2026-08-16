package gb28181

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
)

// PortPool rents UDP/TCP media ports from a configured range.
type PortPool struct {
	mu    sync.Mutex
	min   uint16
	max   uint16
	next  uint16
	inUse map[uint16]struct{}
}

func NewPortPool(rangeSpec string) (*PortPool, error) {
	minP, maxP, err := ParsePortRange(rangeSpec)
	if err != nil {
		return nil, err
	}
	return &PortPool{
		min:   minP,
		max:   maxP,
		next:  minP,
		inUse: make(map[uint16]struct{}),
	}, nil
}

func ParsePortRange(s string) (uint16, uint16, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 10000, 20000, nil
	}
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		p, err := strconv.Atoi(s)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid media_port %q", s)
		}
		return uint16(p), uint16(p), nil
	}
	a, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	b, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || a <= 0 || b < a || b > 65535 {
		return 0, 0, fmt.Errorf("invalid media_port %q", s)
	}
	return uint16(a), uint16(b), nil
}

func (p *PortPool) Acquire() (uint16, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := int(p.max) - int(p.min) + 1
	for i := 0; i < n; i++ {
		port := p.next
		p.next++
		if p.next > p.max {
			p.next = p.min
		}
		if _, used := p.inUse[port]; !used {
			p.inUse[port] = struct{}{}
			return port, nil
		}
	}
	return 0, fferrors.ErrGbMediaPortExhausted
}

func (p *PortPool) Release(port uint16) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.inUse, port)
}
