package trace

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
	"golang.org/x/sync/semaphore"
)

type ICMPTracerv6 struct {
	Config
	wg                  sync.WaitGroup
	res                 Result
	ctx                 context.Context
	inflightRequest     map[int]chan Hop
	inflightRequestLock sync.Mutex
	icmpListen          net.PacketConn
	workFork            workFork
	final               int
	finalLock           sync.Mutex

	sem *semaphore.Weighted
}

func (t *ICMPTracerv6) Execute() (*Result, error) {
	if len(t.res.Hops) > 0 {
		return &t.res, ErrTracerouteExecuted
	}

	var err error

	t.icmpListen, err = net.ListenPacket("ip6:58", "::")
	if err != nil {
		return &t.res, err
	}
	defer t.icmpListen.Close()

	var cancel context.CancelFunc
	t.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	t.inflightRequest = make(map[int]chan Hop)
	t.final = -1

	go t.listenICMP()

	t.sem = semaphore.NewWeighted(int64(t.ParallelRequests))

	for t.workFork.ttl = 1; t.workFork.ttl <= t.MaxHops; t.workFork.ttl++ {
		for i := 0; i < t.NumMeasurements; i++ {
			t.wg.Add(1)
			go t.send(workFork{t.workFork.ttl, i})
		}
		// 一组TTL全部退出（收到应答或者超时终止）以后，再进行下一个TTL的包发送
		t.wg.Wait()
		t.workFork.num = 0
	}
	t.res.reduce(t.final)

	return &t.res, nil
}

func (t *ICMPTracerv6) listenICMP() {
	lc := NewPacketListener(t.icmpListen, t.ctx)
	go lc.Start()
	for {
		select {
		case <-t.ctx.Done():
			return
		case msg := <-lc.Messages:
			if msg.N == nil {
				continue
			}
			rm, err := icmp.ParseMessage(58, msg.Msg[:*msg.N])
			if err != nil {
				log.Println(err)
				continue
			}
			// log.Println(msg.Peer)
			switch rm.Type {
			case ipv6.ICMPTypeTimeExceeded:
				t.handleICMPMessage(msg, 0, rm.Body.(*icmp.TimeExceeded).Data)
			case ipv6.ICMPTypeEchoReply:
				t.handleICMPMessage(msg, 1, rm.Body.(*icmp.Echo).Data)
			default:
				// log.Println("received icmp message of unknown type", rm.Type)
			}
		}
	}

}

func (t *ICMPTracerv6) handleICMPMessage(msg ReceivedMessage, icmpType int8, data []byte) {

	t.inflightRequestLock.Lock()
	defer t.inflightRequestLock.Unlock()
	ch, ok := t.inflightRequest[t.workFork.num]
	t.workFork.num += 1
	if !ok {
		return
	}
	ch <- Hop{
		Success: true,
		Address: msg.Peer,
	}
}

func (t *ICMPTracerv6) send(fork workFork) error {
	err := t.sem.Acquire(context.Background(), 1)
	if err != nil {
		return err
	}

	defer t.sem.Release(1)

	defer t.wg.Done()
	if t.final != -1 && fork.ttl > t.final {
		return nil
	}

	icmpHeader := icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest, Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Data: []byte("HELLO-R-U-THERE"),
		},
	}

	p := ipv6.NewPacketConn(t.icmpListen)

	icmpHeader.Body.(*icmp.Echo).Seq = fork.ttl
	p.SetHopLimit(fork.ttl)

	wb, err := icmpHeader.Marshal(nil)
	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	if _, err := t.icmpListen.WriteTo(wb, &net.IPAddr{IP: t.DestIP}); err != nil {
		log.Fatal(err)
	}
	if err := t.icmpListen.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		log.Fatal(err)
	}
	t.inflightRequestLock.Lock()
	hopCh := make(chan Hop)
	t.inflightRequest[fork.num] = hopCh
	t.inflightRequestLock.Unlock()

	// defer func() {
	// 	t.inflightRequestLock.Lock()
	// 	close(hopCh)
	// 	delete(t.inflightRequest, fork.ttl)
	// 	t.inflightRequestLock.Unlock()
	// }()

	if fork.num == 0 && t.Config.RoutePath {
		fmt.Print(strconv.Itoa(fork.ttl))
	}

	select {
	case <-t.ctx.Done():
		return nil
	case h := <-hopCh:
		rtt := time.Since(start)
		if t.final != -1 && fork.ttl > t.final {
			return nil
		}
		if addr, ok := h.Address.(*net.IPAddr); ok && addr.IP.Equal(t.DestIP) {
			t.finalLock.Lock()
			if t.final == -1 || fork.ttl < t.final {
				t.final = fork.ttl
			}
			t.finalLock.Unlock()
		} else if addr, ok := h.Address.(*net.TCPAddr); ok && addr.IP.Equal(t.DestIP) {
			t.finalLock.Lock()
			if t.final == -1 || fork.ttl < t.final {
				t.final = fork.ttl
			}
			t.finalLock.Unlock()
		}

		h.TTL = fork.ttl
		h.RTT = rtt

		h.fetchIPData(t.Config)
		if t.Config.RoutePath {
			HopPrinter(h)
		}

		t.res.add(h)

	case <-time.After(t.Timeout):
		if t.final != -1 && fork.ttl > t.final {
			return nil
		}

		t.res.add(Hop{
			Success: false,
			Address: nil,
			TTL:     fork.ttl,
			RTT:     0,
			Error:   ErrHopLimitTimeout,
		})
		if t.Config.RoutePath {
			fmt.Println("\t" + "*")
		}
	}

	return nil
}
