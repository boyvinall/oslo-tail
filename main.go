package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/gravitational/version"

	"github.com/urfave/cli"
)

const (
	portHTTP = 80
	portAMQP = 5672
)

var cmdRun = cli.Command{
	Name:   "run",
	Action: run,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name: "interface, i",
		},
		cli.StringFlag{
			Name:  "filter, f",
			Usage: "pcap capture filter",
			Value: "",
		},
		cli.StringFlag{
			Name:  "file, r",
			Usage: "read from file",
		},
		cli.StringFlag{
			Name:  "exchange, x",
			Usage: "show only messages from specified amqp exchange",
		},
		cli.StringFlag{
			Name:  "method, m",
			Usage: "show only messages containing specified oslo method",
		},
	},
}

func run(c *cli.Context) error {

	var handle *pcap.Handle
	var err error

	intf := c.String("interface")
	file := c.String("file")

	switch {
	case intf != "" && file != "":
		return errors.New("please specify either interface or file, not both")

	case intf != "":
		handle, err = pcap.OpenLive(intf, 65535, true, pcap.BlockForever)
		if err != nil {
			return err
		}

	case file != "":
		handle, err = pcap.OpenOffline(file)
		if err != nil {
			return err
		}
	}

	filter := c.String("filter")
	if filter != "" {
		if err := handle.SetBPFFilter(filter); err != nil {
			return err
		}
	}

	d := NewStreamDemux()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packets := packetSource.Packets()
	ticker := time.Tick(time.Minute)
	for {
		select {
		case packet := <-packets:
			// A nil packet indicates the end of a pcap file.
			if packet == nil {
				return nil
			}
			flow := GetPacketFlow(packet).String()
			if flow == "" {
				continue
			}
			i := d.Get(flow, func(s *stream) {
				a := amqpDecoder{
					filterExchange: c.String("exchange"),
					filterMethod:   c.String("method"),
				}
				go func() {
					for {
						err := a.decodeAMQP(s)
						if err == io.EOF {
							return
						}
						if err != nil {
							fmt.Fprintln(os.Stderr, "ERROR: decode:", err.Error())
						}
					}
				}()

			})
			i.s.Write(packet)

		case <-ticker:
			d.CloseInactive()
		}
	}

	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "oslo-tail"
	app.Usage = "decode openstack OSLO messages"
	app.Version = version.Get().Version
	app.EnableBashCompletion = true
	app.CommandNotFound = func(c *cli.Context, cmd string) {
		fmt.Fprintf(os.Stderr, "ERROR: Unknown command '%s'\n", cmd)
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name: "interface, i",
		},
	}
	app.Commands = []cli.Command{
		cmdRun,
	}
	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %+v\n", err)
		os.Exit(1)
	}
}
