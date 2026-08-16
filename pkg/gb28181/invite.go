package gb28181

import (
	"sync"
)

// InviteSession represents an active INVITE dialog for live play.
type InviteSession struct {
	DeviceID  string
	ChannelID string
	CallID    string
	SSRC      string
	Media     *MediaReceiver

	mu     sync.Mutex
	bye    func() error
	closed bool
}

func (s *InviteSession) SetBye(fn func() error) {
	s.mu.Lock()
	s.bye = fn
	s.mu.Unlock()
}

func (s *InviteSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if s.bye != nil {
		err = s.bye()
	}
	if s.Media != nil {
		_ = s.Media.Close()
	}
	return err
}
