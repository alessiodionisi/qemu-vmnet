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
)

func main() {
	vmn := vmnet.New()

	if err := vmn.Start(); err != nil {
		fmt.Printf("unable to start vmnet interface, please try again with \"sudo\"\n")
		os.Exit(1)
		return
	}
	defer vmn.Stop()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: 1234,
	})
	if err != nil {
		fmt.Printf("unable to start the udp connection, %s\n", err.Error())
		os.Exit(1)
		return
	}
	defer conn.Close()

	writeToClientsChan := make(chan []byte)
	writeToVNNetChan := make(chan []byte)
	udpAddrs := map[string]*net.UDPAddr{}

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

			writeToClientsChan <- bytes
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
			bytesLen, udpAddr, err := conn.ReadFromUDP(bytes)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					break
				}

				log.Printf("error while reading from %s: %s\n", udpAddr.String(), err.Error())
				continue
			}

			_, exist := udpAddrs[udpAddr.String()]
			if !exist {
				udpAddrs[udpAddr.String()] = udpAddr
			}

			bytes = bytes[:bytesLen]

			log.Printf("received %d bytes from %s\n", bytesLen, udpAddr.String())

			writeToVNNetChan <- bytes
		}
	}()

	go func() {
		for {
			bytes := <-writeToClientsChan

			for _, udpAddr := range udpAddrs {
				log.Printf("writing %d bytes to %s\n", len(bytes), udpAddr.String())

				if _, err := conn.WriteToUDP(bytes, udpAddr); err != nil {
					if errors.Is(err, net.ErrClosed) {
						delete(udpAddrs, udpAddr.String())
						log.Printf("connection from %s closed\n", udpAddr.String())
						continue
					}

					log.Printf("error while writing to %s: %s\n", udpAddr.String(), err.Error())
					continue
				}
			}
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
