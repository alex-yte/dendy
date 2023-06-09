package netplay

import (
	"fmt"
	"net"
)

const (
	inputBatchSize = 5
)

type Netplay struct {
	game       *Game
	toRecv     chan Message
	toSend     chan Message
	stop       chan struct{}
	inputBatch InputBatch
	remoteConn net.Conn
}

func Listen(game *Game, addr string) (*Netplay, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("netplay: failed to listen on %s: %v", addr, err)
	}

	conn, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("netplay: failed to accept connection: %v", err)
	}

	return &Netplay{
		toSend:     make(chan Message, 1000),
		toRecv:     make(chan Message, 1000),
		stop:       make(chan struct{}),
		game:       game,
		remoteConn: conn,
	}, nil
}

func Connect(game *Game, addr string) (*Netplay, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("netplay: failed to connect to %s: %v", addr, err)
	}

	return &Netplay{
		toSend:     make(chan Message, 1000),
		toRecv:     make(chan Message, 1000),
		stop:       make(chan struct{}),
		game:       game,
		remoteConn: conn,
	}, nil
}

func (np *Netplay) startWriter() {
	for {
		select {
		case <-np.stop:
			return
		case msg := <-np.toSend:
			if err := writeMsg(np.remoteConn, msg); err != nil {
				panic(fmt.Errorf("failed to write message: %v", err))
			}
		}
	}
}

func (np *Netplay) startReader() {
	for {
		select {
		case <-np.stop:
			return
		default:
			msg, err := readMsg(np.remoteConn)
			if err != nil {
				panic(fmt.Errorf("failed to read message: %v", err))
			}

			np.toRecv <- msg
		}
	}
}

func (np *Netplay) handleMessage(msg Message) bool {
	switch msg.Type {
	case MsgTypeReset:
		np.resetInputBatch(msg.Frame)
		np.game.Reset(&Checkpoint{
			Frame: msg.Frame,
			State: msg.Payload,
		})
		return false

	case MsgTypeInput:
		np.game.AddRemoteInput(InputBatch{
			Input:      msg.Payload,
			StartFrame: msg.Frame,
		})
	}

	return true
}

func (np *Netplay) Start() {
	go np.startReader()
	go np.startWriter()
}

func (np *Netplay) resetInputBatch(startFrame uint64) {
	np.inputBatch = InputBatch{
		StartFrame: startFrame,
		Input:      make([]uint8, 0, inputBatchSize),
	}
}

// SendReset restarts the game on both sides, should be called by the server once the
// game is ready to start to sync the initial state.
func (np *Netplay) SendReset() {
	np.game.Reset(nil)
	np.resetInputBatch(0)
	cp := np.game.Checkpoint()

	np.toSend <- Message{
		Type:    MsgTypeReset,
		Frame:   cp.Frame,
		Payload: cp.State,
	}
}

// SendInput sends the local input to the remote player. Should be called every frame.
// The input is buffered and sent in batches to reduce the number of messages sent.
func (np *Netplay) SendInput(buttons uint8) {
	np.game.AddLocalInput(buttons)
	np.inputBatch.Add(buttons)

	if np.inputBatch.Len() >= inputBatchSize {
		np.toSend <- Message{
			Type:    MsgTypeInput,
			Payload: np.inputBatch.Input,
			Frame:   np.inputBatch.StartFrame,
		}

		np.inputBatch = InputBatch{
			StartFrame: np.game.Frame() + 1,
			Input:      make([]uint8, 0, inputBatchSize),
		}
	}
}

func (np *Netplay) RunFrame() {
	select {
	case msg := <-np.toRecv:
		np.handleMessage(msg)
	default:
	}

	np.game.RunFrame()
}