package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type msgDecoder interface {
	decode(b []byte) error
	String() string
}

type msgBasicDeliver struct {
	ConsumerTag string
	DeliveryTag uint64
	Redelivered bool
	Exchange    string
	RoutingKey  string
}

func (m *msgBasicDeliver) decode(b []byte) error {
	buf := bytes.NewBuffer(b)

	m.ConsumerTag = shortstr(buf)

	err := binary.Read(buf, binary.BigEndian, &m.DeliveryTag)
	if err != nil {
		return err
	}

	bits, err := buf.ReadByte()
	if err != nil {
		return err
	}
	m.Redelivered = (bits&(1<<0) > 0)

	m.Exchange = shortstr(buf)
	m.RoutingKey = shortstr(buf)

	return nil
}

func (m *msgBasicDeliver) String() string {
	return fmt.Sprintf("x=%s rk=%s consumerTag=%s deliveryTag=%v", m.Exchange, m.RoutingKey, m.ConsumerTag, m.DeliveryTag)
}

type msgBasicAck struct {
	DeliveryTag uint64
	Multiple    bool
}

func (m *msgBasicAck) decode(b []byte) error {
	buf := bytes.NewBuffer(b)

	err := binary.Read(buf, binary.BigEndian, &m.DeliveryTag)
	if err != nil {
		return err
	}

	bits, err := buf.ReadByte()
	if err != nil {
		return err
	}
	m.Multiple = (bits&(1<<0) > 0)

	return nil
}

func (m *msgBasicAck) String() string {
	return fmt.Sprintf("deliveryTag=%v", m.DeliveryTag)
}

type msgBasicPublish struct {
	Reserved1  uint16
	Exchange   string
	RoutingKey string
	Mandatory  bool
	Immediate  bool
}

func (m *msgBasicPublish) decode(b []byte) error {
	buf := bytes.NewBuffer(b)

	err := binary.Read(buf, binary.BigEndian, &m.Reserved1)
	if err != nil {
		return err
	}

	m.Exchange = shortstr(buf)
	m.RoutingKey = shortstr(buf)

	var bits byte
	bits, err = buf.ReadByte()
	if err != nil {
		return err
	}

	m.Mandatory = (bits&(1<<0) > 0)
	m.Immediate = (bits&(1<<1) > 0)

	return nil
}

func (m *msgBasicPublish) String() string {
	return fmt.Sprintf("x=%s rk=%s", m.Exchange, m.RoutingKey)
}
