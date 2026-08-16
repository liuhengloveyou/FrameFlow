package core

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReceiverLateJoinerSeesPackets(t *testing.T) {
	recv := NewReceiver(nil, &Codec{Name: CodecH264, ClockRate: 90000})

	var early, late atomic.Int64
	earlySender := NewSender(nil, recv.Codec)
	earlySender.Handler = func(*Packet) { early.Add(1) }
	earlySender.HandleRTP(recv)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				recv.WriteRTP(&Packet{Payload: []byte{0, 0, 0, 1, 0x65}})
			}
		}
	}()

	time.Sleep(20 * time.Millisecond)

	lateSender := NewSender(nil, recv.Codec)
	lateSender.Handler = func(*Packet) { late.Add(1) }
	lateSender.HandleRTP(recv)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if late.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(stop)
	wg.Wait()

	require.Greater(t, early.Load(), int64(0), "early sender should receive packets")
	require.Greater(t, late.Load(), int64(0), "late joiner must see fan-out after AppendChild")
}

func TestSenser(t *testing.T) {
	recv := make(chan *Packet) // blocking receiver

	sender := NewSender(nil, &Codec{})
	sender.Output = func(packet *Packet) {
		recv <- packet
	}
	require.Equal(t, "new", sender.State())

	sender.Start()
	require.Equal(t, "connected", sender.State())

	sender.Input(&Packet{})
	sender.Input(&Packet{})

	require.Equal(t, 2, sender.Packets)
	require.Equal(t, 0, sender.Drops)

	// important to read one before close
	// because goroutine in Start() can run with nil chan
	// it's OK in real life, but bad for test
	_, ok := <-recv
	require.True(t, ok)

	sender.Close()
	require.Equal(t, "closed", sender.State())

	sender.Input(&Packet{})

	require.Equal(t, 2, sender.Packets)
	require.Equal(t, 1, sender.Drops)

	// read 2nd
	_, ok = <-recv
	require.True(t, ok)

	// read 3rd
	select {
	case <-recv:
		ok = true
	default:
		ok = false
	}
	require.False(t, ok)
}
