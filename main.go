package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/adnsio/qemu-vmnet/vmnet"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func main() {
	vmn := vmnet.New()

	if err := vmn.Start(); err != nil {
		fmt.Printf("unable to start vmnet interface, please try again with \"sudo\"\n")
		os.Exit(1)
		return
	}
	defer vmn.Stop()

	conn, err := net.ListenPacket("udp", ":1234")
	if err != nil {
		fmt.Printf("unable to start the listener, %s\n", err.Error())
		os.Exit(1)
		return
	}
	defer conn.Close()

	writeToVNNetChan := make(chan []byte)
	clients := map[string]net.Addr{}

	go func() {
		for {
			bytes := make([]byte, vmn.MaxPacketSize)
			bytesLen, err := vmn.Read(bytes)
			if err != nil {
				log.Printf("error while reading from vmnet: %s\n", err.Error())
				continue
			}

			bytes = bytes[:bytesLen]
			log.Printf("received %d bytes from vmnet\n", bytesLen)

			go func(bytes []byte) {
				pkt := gopacket.NewPacket(bytes, layers.LayerTypeEthernet, gopacket.Default)
				// log.Printf("%s\n", pkt.String())

				layer := pkt.Layer(layers.LayerTypeEthernet)
				if layer == nil {
					return
				}

				ethLayer, _ := layer.(*layers.Ethernet)
				destinationMAC := ethLayer.DstMAC.String()

				addr, exist := clients[destinationMAC]
				if !exist {
					return
				}

				log.Printf("writing %d bytes to %s\n", len(bytes), addr.String())

				if _, err := conn.WriteTo(bytes, addr); err != nil {
					if errors.Is(err, net.ErrClosed) {
						delete(clients, destinationMAC)
						log.Printf("deleted client with mac %s\n", destinationMAC)
						return
					}

					log.Printf("error while writing to %s: %s\n", addr.String(), err.Error())
					return
				}
			}(bytes)
		}
	}()

	go func() {
		for {
			bytes := <-writeToVNNetChan

			log.Printf("writing %d bytes to vmnet\n", len(bytes))

			if _, err := vmn.Write(bytes); err != nil {
				log.Printf("error while writing to vmnet: %s\n", err.Error())
				continue
			}
		}
	}()

	go func() {
		for {
			bytes := make([]byte, vmn.MaxPacketSize)
			bytesLen, addr, err := conn.ReadFrom(bytes)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					break
				}

				log.Printf("error while reading from %s: %s\n", addr.String(), err.Error())
				continue
			}

			bytes = bytes[:bytesLen]
			pkt := gopacket.NewPacket(bytes, layers.LayerTypeEthernet, gopacket.Default)

			log.Printf("received %d bytes from %s\n", bytesLen, addr.String())
			// log.Printf("%s\n", pkt.String())

			if layer := pkt.Layer(layers.LayerTypeEthernet); layer != nil {
				eth, _ := layer.(*layers.Ethernet)

				_, exist := clients[eth.SrcMAC.String()]
				if !exist {
					clients[eth.SrcMAC.String()] = addr
					log.Printf("new client with mac %s\n", eth.SrcMAC.String())
				}

				writeToVNNetChan <- bytes
			}
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
