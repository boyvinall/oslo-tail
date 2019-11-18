package main

type stream struct {
	b       []byte
	ch      chan []byte
	packets int
}

func NewStream() *stream {
	s := &stream{
		ch: make(chan []byte, 10),
	}
	return s
}

func (s *stream) Write(b []byte) {
	if len(b) > 0 {
		s.ch <- b
	}
}

func (s *stream) Read(p []byte) (n int, err error) {
	if len(s.b) == 0 {
		s.b = <-s.ch
		s.packets++
		// fmt.Printf("- packet %d = %d bytes\n", s.packets, len(s.b))
	}
	written := copy(p, s.b)
	s.b = s.b[written:]
	return written, nil
}

func (s *stream) Packets() int {
	return s.packets
}
