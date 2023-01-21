package main

import (
	"errors"
	"flag"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"sync"
	"syscall"

	"github.com/alessiodionisi/qemu-vmnet/pkg/vmnet"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	debug := flag.Bool("debug", false, "sets log level to debug")
	trace := flag.Bool("trace", false, "sets log level to trace")
	address := flag.String("address", ":2233", "sets the listening address")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal().Msgf("could not create CPU profile: %s", err.Error())
			return
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal().Msgf("could not start CPU profile: %s", err.Error())
			return
		}
		defer pprof.StopCPUProfile()
	}

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	if *trace {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
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
	clientsMutex := sync.Mutex{}

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
				log.Debug().Msgf("received %d bytes from vmnet", len(bytes))
				log.Trace().Msg(pkt.String())

				layer := pkt.Layer(layers.LayerTypeEthernet)
				if layer == nil {
					return
				}

				ethLayer, _ := layer.(*layers.Ethernet)
				destinationMAC := ethLayer.DstMAC.String()

				clientsMutex.Lock()
				addr, exist := clients[destinationMAC]
				clientsMutex.Unlock()
				if !exist {
					return
				}

				log.Debug().Msgf("writing %d bytes to %s", len(bytes), destinationMAC)

				if _, err := conn.WriteTo(bytes, addr); err != nil {
					if errors.Is(err, net.ErrClosed) {
						clientsMutex.Lock()
						delete(clients, destinationMAC)
						clientsMutex.Unlock()
						log.Debug().Msgf("deleted client with mac %s", destinationMAC)
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

			log.Debug().Msgf("writing %d bytes to vmnet", len(bytes))

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

				if layer := pkt.Layer(layers.LayerTypeEthernet); layer != nil {
					eth, _ := layer.(*layers.Ethernet)
					sourceMAC := eth.SrcMAC.String()

					log.Debug().Msgf("received %d bytes from %s", len(bytes), sourceMAC)
					log.Trace().Msg(pkt.String())

					clientsMutex.Lock()
					_, exist := clients[sourceMAC]
					clientsMutex.Unlock()
					if !exist {
						clientsMutex.Lock()
						clients[sourceMAC] = addr
						clientsMutex.Unlock()
						log.Debug().Msgf("new client with mac %s", sourceMAC)
					}

					writeToVNNetChan <- bytes
				}
			}(bytes)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal().Msgf("could not create memory profile: %s", err.Error())
			return
		}
		defer f.Close()

		runtime.GC()

		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal().Msgf("could not write memory profile: %s", err.Error())
			return
		}
	}
}
