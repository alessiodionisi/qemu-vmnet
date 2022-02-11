package main

import (
	"errors"
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/adnsio/qemu-vmnet/pkg/vmnet"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	debug := flag.Bool("debug", false, "sets log level to debug")
	address := flag.String("address", ":2233", "sets the listening address")

	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	vmn := vmnet.New()

	if err := vmn.Start(); err != nil {
		log.Fatal().Msg("unable to start vmnet interface, please try again with \"sudo\"")
		return
	}
	defer vmn.Stop()

	conn, err := net.ListenPacket("udp", *address)
	if err != nil {
		log.Fatal().Msgf("unable to start the listener, %s", err.Error())
		return
	}
	defer conn.Close()

	log.Info().Msgf("listening on %s", conn.LocalAddr())

	writeToVNNetChan := make(chan []byte)
	clients := map[string]net.Addr{}

	go func() {
		for {
			bytes := make([]byte, vmn.MaxPacketSize)
			bytesLen, err := vmn.Read(bytes)
			if err != nil {
				log.Error().Msgf("error while reading from vmnet: %s", err.Error())
				continue
			}

			bytes = bytes[:bytesLen]

			go func(bytes []byte) {
				pkt := gopacket.NewPacket(bytes, layers.LayerTypeEthernet, gopacket.Default)
				log.Debug().Msgf("received %d bytes from vmnet\n%s", len(bytes), pkt.String())

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

				log.Info().Msgf("writing %d bytes to %s", len(bytes), addr.String())

				if _, err := conn.WriteTo(bytes, addr); err != nil {
					if errors.Is(err, net.ErrClosed) {
						delete(clients, destinationMAC)
						log.Info().Msgf("deleted client with mac %s", destinationMAC)
						return
					}

					log.Error().Msgf("error while writing to %s: %s", addr.String(), err.Error())
					return
				}
			}(bytes)
		}
	}()

	go func() {
		for {
			bytes := <-writeToVNNetChan

			log.Info().Msgf("writing %d bytes to vmnet", len(bytes))

			if _, err := vmn.Write(bytes); err != nil {
				log.Error().Msgf("error while writing to vmnet: %s", err.Error())
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

				log.Error().Msgf("error while reading from %s: %s", addr.String(), err.Error())
				continue
			}

			bytes = bytes[:bytesLen]

			go func(bytes []byte) {
				pkt := gopacket.NewPacket(bytes, layers.LayerTypeEthernet, gopacket.Default)
				log.Debug().Msgf("received %d bytes from %s\n%s", len(bytes), addr.String(), pkt.String())

				if layer := pkt.Layer(layers.LayerTypeEthernet); layer != nil {
					eth, _ := layer.(*layers.Ethernet)

					_, exist := clients[eth.SrcMAC.String()]
					if !exist {
						clients[eth.SrcMAC.String()] = addr
						log.Info().Msgf("new client with mac %s", eth.SrcMAC.String())
					}

					writeToVNNetChan <- bytes
				}
			}(bytes)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
