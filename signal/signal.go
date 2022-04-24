package signal

type Signal struct {
	sigChan chan struct{}
}

func New() *Signal {
	return &Signal{sigChan: make(chan struct{}, 1)}
}

func (s *Signal) Signal() {
	if len(s.sigChan) == 0 {
		s.sigChan <- struct{}{}
	}
}

func (s *Signal) Chan() chan struct{} {
	return s.sigChan
}
