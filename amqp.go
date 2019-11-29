package main

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

type frameType uint8

func (t frameType) String() string {
	s := frameTypeLookup[t]
	if s != "" {
		return s
	}
	return fmt.Sprintf("unknown(%d)", int(t))
}

func (t frameType) IsValid() bool {
	switch t {
	case frameMethod, frameHeader, frameBody, frameHeartbeat:
		return true
	default:
		return false
	}
}

var frameTypeLookup = map[frameType]string{
	frameMethod:    "method",
	frameHeader:    "header",
	frameBody:      "body",
	frameHeartbeat: "heartbeart",
}

const (
	frameMethod    frameType = 1
	frameHeader    frameType = 2
	frameBody      frameType = 3
	frameHeartbeat frameType = 8
)

const (
	frameEnd = 206
	maxSize  = 1024 * 100
)

type amqpDecoder struct {
	filterExchange string
	filterMethod   string

	flow         string
	display      bool
	basicDeliver *msgBasicDeliver
	info         packetInfo
}

func (a *amqpDecoder) decodeAMQP(s *stream) error {
	r := io.Reader(s)
	var scratch [7]byte

	_, err := io.ReadFull(r, scratch[:7])
	a.info = s.Info()
	log := a.info.Log()

	if err != nil {
		return errors.Wrap(err, log)
	}

	flow := GetPacketFlow(a.info.p).String()
	if flow != a.flow {
		return errors.Errorf("mismatched flow %s != %s ??", flow, a.flow)
	}

	typ := frameType(scratch[0])
	channel := binary.BigEndian.Uint16(scratch[1:3])
	size := int(binary.BigEndian.Uint32(scratch[3:7]))

	if size > maxSize || !typ.IsValid() {
		b := make([]byte, maxSize)
		n, _ := r.Read(b)
		b = append(scratch[:], b[:n]...)
		if len(b) > 256 {
			b = b[:256]
		}
		err := errors.New(hex.Dump(b))
		err = errors.Wrapf(err, "typ=%s size=%d n=%d packets=%d\n", typ, size, n, s.Packets())
		return errors.Wrap(err, log)
	}
	// fmt.Printf(log)

	b := make([]byte, size)
	n, err := io.ReadFull(r, b)
	if n != size {
		return fmt.Errorf("read only %d bytes instead of expected %d", n, size)
	}
	if err != nil {
		return errors.Wrap(err, log)
	}

	switch typ {
	case frameMethod:
		err = a.decodeMethod(b)
		if err != nil {
			return errors.Wrap(err, log)
		}

	case frameBody:
		err = a.decodeBody(b)
		if err != nil {
			return errors.Wrap(err, log)
		}

	case frameHeader:

	default:
		if a.filterExchange == "" && a.filterMethod == "" {
			fmt.Printf("%-30s: channel=%d size=%d\n", typ, channel, size)
		}
	}

	if _, err := io.ReadFull(r, scratch[:1]); err != nil {
		return errors.Wrap(err, log)
	}

	if scratch[0] != frameEnd {
		err := errors.New("end byte invalid")
		return errors.Wrap(err, log)
	}

	return nil
}

type classID uint16

var classLookup = map[classID]string{
	classBasic: "basic",
}

func (c classID) String() string {
	s := classLookup[c]
	if s != "" {
		return s
	}
	return fmt.Sprintf("unknown(%d)", int(c))
}

type methodID uint16

var methodLookup = map[methodID]string{
	methodDeliver: "deliver",
	methodAck:     "ack",
	methodPublish: "publish",
}

func (m methodID) String() string {
	s := methodLookup[m]
	if s != "" {
		return s
	}
	return fmt.Sprintf("unknown(%d)", int(m))
}

const (
	classBasic classID = 60

	methodPublish methodID = 40
	methodDeliver methodID = 60
	methodAck     methodID = 80
)

func (a *amqpDecoder) decodeMethod(b []byte) error {
	c := classID(binary.BigEndian.Uint16(b[0:2]))
	m := methodID(binary.BigEndian.Uint16(b[2:4]))

	a.display = true
	var msg msgDecoder
	switch c {
	case classBasic:
		switch m {
		case methodDeliver:
			a.basicDeliver = &msgBasicDeliver{}
			msg = a.basicDeliver
			err := msg.decode(b[4:])
			if err != nil {
				return err
			}

			a.display = a.filterExchange == "" || a.filterExchange == a.basicDeliver.Exchange

		case methodAck:
			msg = &msgBasicAck{}
			err := msg.decode(b[4:])
			if err != nil {
				return err
			}

			a.display = a.filterExchange == ""

		case methodPublish:
			msg = &msgBasicPublish{}
			err := msg.decode(b[4:])
			if err != nil {
				return err
			}

			a.display = a.filterExchange == ""
			// a.display = a.filterExchange == "" || a.filterExchange == a.basicDeliver.Exchange
		}
	}

	var extra string
	if msg != nil {
		extra = ": " + msg.String()
	}

	if a.display && a.filterMethod == "" {
		fmt.Printf(a.info.Log())
		fmt.Printf("%-30s%s\n", fmt.Sprintf("%s.%s", c, m), extra)
	}
	return nil
}

func (a *amqpDecoder) decodeBody(b []byte) error {
	if !a.display {
		return nil
	}

	m := map[string]interface{}{}
	err := json.Unmarshal(b, &m)
	if err != nil {
		return errors.Wrap(err, "unmarshal/1")
	}
	msg := m["oslo.message"]
	if msg == "" {
		return nil
	}

	payload := map[string]interface{}{}
	bb := []byte(msg.(string))
	err = json.Unmarshal(bb, &payload)
	if err != nil {
		return errors.Wrap(err, "unmarshal/2")
	}
	_, methodOK := payload["method"]
	_, resultOK := payload["result"]
	if methodOK {
		if a.filterMethod == "" {
			p, err := json.MarshalIndent(payload["args"], "  ", "  ")
			if err != nil {
				return errors.Wrap(err, "marshal/1")
			}
			fmt.Println(" ", payload["method"].(string), string(p))
		} else if a.filterMethod == payload["method"].(string) {
			p, err := json.MarshalIndent(payload["args"], "", "  ")
			if err != nil {
				return errors.Wrap(err, "marshal/2")
			}
			fmt.Printf("%s method=%s\n%s\n", a.basicDeliver.String(), payload["method"].(string), string(p))
		}
	} else if resultOK {
		// fmt.Printf("%-30s : %v\n", "", "(result)")
	} else {
		// fmt.Println(msg.(string))
	}
	return nil
}

func shortstr(r io.Reader) string {
	var lb [1]byte
	n, err := r.Read(lb[:])
	if n != 1 || err != nil {
		return ""
	}
	len := int(lb[0])
	b := make([]byte, len)
	n, err = r.Read(b)
	if n != len || err != nil {
		return ""
	}
	return string(b)
}
