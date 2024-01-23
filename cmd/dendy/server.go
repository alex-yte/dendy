package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/maxpoletaev/dendy/console"
	"github.com/maxpoletaev/dendy/input"
	"github.com/maxpoletaev/dendy/netplay"
	"github.com/maxpoletaev/dendy/ui"
)

func runAsServer(bus *console.Bus, o *opts, saveFile string) {
	bus.Joy1 = input.NewJoystick()
	bus.Joy2 = input.NewJoystick()
	bus.InitDMA()
	bus.Reset()

	if !o.noSave {
		if ok, err := loadState(bus, saveFile); err != nil {
			log.Printf("[ERROR] failed to load save state: %s", err)
			os.Exit(1)
		} else if ok {
			log.Printf("[INFO] state loaded: %s", saveFile)
		}
	}

	game := netplay.NewGame(bus)
	game.RemoteJoy = bus.Joy2
	game.LocalJoy = bus.Joy1
	game.Init(nil)

	if o.disasm != "" {
		file, err := os.Create(o.disasm)
		if err != nil {
			log.Printf("[ERROR] failed to create disassembly file: %s", err)
			os.Exit(1)
		}

		writer := bufio.NewWriterSize(file, 1024*1024)

		defer func() {
			flushErr := writer.Flush()
			closeErr := file.Close()

			if err := errors.Join(flushErr, closeErr); err != nil {
				log.Printf("[ERROR] failed to close disassembly file: %s", err)
			}
		}()

		bus.DisasmWriter = writer
		bus.DisasmEnabled = false // will be controlled by the game
		game.DisasmEnabled = true
	}

	log.Printf("[INFO] waiting for client...")
	sess, addr, err := netplay.Listen(game, o.listenAddr)

	if err != nil {
		log.Printf("[ERROR] failed to listen: %v", err)
		os.Exit(1)
	}

	log.Printf("[INFO] client connected: %s", addr)
	log.Printf("[INFO] starting game...")

	sess.SendInitialState()

	w := ui.CreateWindow(&bus.PPU.Frame, o.scale, o.verbose)
	defer w.Close()

	w.SetTitle(fmt.Sprintf("%s (P1)", windowTitle))
	w.SetFrameRate(framesPerSecond)
	w.ResyncDelegate = sess.SendResync
	w.InputDelegate = sess.SendButtons
	w.ResetDelegate = sess.SendReset
	w.ShowFPS = o.showFPS
	w.ShowPing = true

	defer func() {
		if o.noSave {
			return
		}

		// Keep the game state in case of a crash (as the netplay is still unstable).
		if err := recover(); err != nil {
			_ = saveState(bus, saveFile)
			panic(err)
		}
	}()

	for {
		startTime := time.Now()

		if w.ShouldClose() {
			log.Printf("[INFO] saying goodbye...")
			sess.SendBye()
			break
		}

		if sess.ShouldExit() {
			log.Printf("[INFO] client disconnected")
			break
		}

		w.HandleHotKeys()
		w.UpdateJoystick()
		w.SetGrayscale(game.Sleeping())
		w.SetPingInfo(sess.RemotePing())

		sess.HandleMessages()
		sess.RunFrame(startTime)

		w.Refresh()
	}

	if !o.noSave {
		if err := saveState(bus, saveFile); err != nil {
			log.Printf("[ERROR] failed to save state: %s", err)
			os.Exit(1)
		}

		log.Printf("[INFO] state saved: %s", saveFile)
	}
}
