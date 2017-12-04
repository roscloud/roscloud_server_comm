package main

import (
	"github.com/gorilla/websocket"
	"log"
	"time"
)

type sendChannel struct {
	r    *receiver
	data []byte
}

type sender struct {
	ws          *websocket.Conn
	h           *hub
	send        chan sendChannel
	name        string
	shortname   string
	privateKey  string
	description string
	freqs       map[*receiver]float32
}

type senderMessage struct {
	PrivateKey  string
	Description string
	Name        string
	Freqs       map[string]float32
}

// heartbeat from sender sends a heartbeat packet to the client every second,
// when no regular packets are being sent.
func (s *sender) heartbeat(beat chan bool, stop chan bool) {
	for {
		exit := false
		select {
		case <-beat:
		case <-time.After(500 * time.Millisecond):
			message := make([]byte, 1)
			s.send <- sendChannel{r: &receiver{}, data: message}
		case <-stop:
			exit = true
		}
		if exit {
			break
		}
	}
}

// writer from sender writes packets to the receiving client.
// It also calculates write frequencies.
func (s *sender) writer() {
	hbeat := make(chan bool)
	stop := make(chan bool)
	go s.heartbeat(hbeat, stop)
	count := make(map[*receiver]int32)
	lastTime := make(map[*receiver]time.Time)
	if db != None {
		s.h.dbw.addKey(false, "clients:"+s.name+":privateKey", s.privateKey)
		s.h.dbw.addKey(false, "clients:"+s.name+":description", s.description)
		s.h.dbw.addKey(false, "clients:"+s.name+":name", s.shortname)
	}

	for message := range s.send {
		select {
		case hbeat <- true:
		default:
		}

		err := s.ws.WriteMessage(websocket.BinaryMessage, message.data)
		if err != nil {
			log.Printf("[%s] WriteError: %s", s.name, err)
			break
		}
		if _, ok := count[message.r]; !ok {
			count[message.r] = 0
			lastTime[message.r] = time.Now()
		}
		count[message.r]++
		if count[message.r] >= 20 {
			s.freqs[message.r] = 20.0 * 1e9 / float32(
				(time.Now().Sub(lastTime[message.r])))
			lastTime[message.r] = time.Now()
			count[message.r] = 0
		}

		if db != None {
			s.h.dbw.addKey(false, "clients:"+s.name+":privateKey", s.privateKey)
			s.h.dbw.addKey(false, "clients:"+s.name+":description", s.description)
			s.h.dbw.addKey(false, "clients:"+s.name+":name", s.shortname)
			for key, value := range s.freqs {
				s.h.dbw.addKey(false, "clients:"+s.name+":freq:"+
					key.name[len("/"+key.privateKey):], value)
			}
		}
	}
	stop <- true
	s.ws.Close()
}
