/*
	Copyright (C) 2021-2022  The YNOproject Developers

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

const (
	delim  = "\uffff"
	mdelim = "\ufffe"
)

var (
	delimBytes = []byte("\uffff")
)

var (
	hubClients sync.Map
	upgrader   = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	hubs []*Hub
)

type ConnInfo struct {
	Connect *websocket.Conn
	Ip      string
	Token   string
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients sync.Map

	// Inbound messages from the clients.
	processMsgCh chan *Message

	// Connection requests from the clients.
	connect chan *ConnInfo

	// Unregister requests from clients.
	unregister chan *Client

	roomId       int
	singleplayer bool

	conditions []*Condition

	minigameConfigs []*MinigameConfig
}

func createAllHubs(roomIds []int, spRooms []int) {
	for _, roomId := range roomIds {
		addHub(roomId, contains(spRooms, roomId))
	}
}

func addHub(roomId int, singleplayer bool) {
	hub := newHub(roomId, singleplayer)
	hubs = append(hubs, hub)
	go hub.run()
}

func newHub(roomId int, singleplayer bool) *Hub {
	return &Hub{
		processMsgCh:    make(chan *Message, 16),
		connect:         make(chan *ConnInfo, 4),
		unregister:      make(chan *Client, 4),
		roomId:          roomId,
		singleplayer:    singleplayer,
		conditions:      getHubConditions(roomId),
		minigameConfigs: getHubMinigameConfigs(roomId),
	}
}

func (h *Hub) run() {
	http.HandleFunc("/"+strconv.Itoa(h.roomId), h.serve)
	for {
		select {
		case conn := <-h.connect:
			client := &Client{
				hub:         h,
				conn:        conn.Connect,
				send:        make(chan []byte, 16),
				key:         generateKey(),
				pictures:    make(map[int]*Picture),
				mapId:       fmt.Sprintf("%04d", h.roomId),
				switchCache: make(map[int]bool),
				varCache:    make(map[int]int),
			}

			var uuid string
			if conn.Token != "" {
				uuid = getUuidFromToken(conn.Token)
			}

			if uuid == "" {
				uuid, _, _ = getOrCreatePlayerData(conn.Ip)
			}

			if s, ok := sessionClients.Load(uuid); ok {
				session := s.(*SessionClient)

				if session.bound {
					writeErrLog(conn.Ip, strconv.Itoa(h.roomId), "session in use")
					continue
				}

				session.bound = true

				client.session = session
			} else {
				writeErrLog(conn.Ip, strconv.Itoa(h.roomId), "player has no session")
				continue
			}

			for {
				client.id++

				if _, ok := h.clients.Load(client.id); !ok {
					break
				}
			}

			if tags, err := getPlayerTags(uuid); err != nil {
				writeErrLog(conn.Ip, strconv.Itoa(h.roomId), "failed to read player tags")
			} else {
				client.tags = tags
			}

			go client.writePump()
			go client.readPump()

			client.sendMsg("s", client.id, int(client.key), uuid, client.session.rank, client.session.account, client.session.badge) // "your id is %id%" message

			// register client in the structures
			h.clients.Store(client.id, client)
			hubClients.Store(uuid, client)

			writeLog(conn.Ip, strconv.Itoa(h.roomId), "connect", 200)
		case client := <-h.unregister:
			client.sendClosed = true
			close(client.send)

			client.session.bound = false

			h.clients.Delete(client.id)
			hubClients.Delete(client.session.uuid)

			h.broadcast("d", client.id) // user %id% has disconnected message

			writeLog(client.session.ip, strconv.Itoa(h.roomId), "disconnect", 200)
		case message := <-h.processMsgCh:
			if errs := h.processMsgs(message); len(errs) > 0 {
				for _, err := range errs {
					writeErrLog(message.sender.session.ip, strconv.Itoa(h.roomId), err.Error())
				}
			}
		}
	}
}

// serve handles websocket requests from the peer.
func (h *Hub) serve(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, http.Header{"Sec-Websocket-Protocol": {r.Header.Get("Sec-Websocket-Protocol")}})
	if err != nil {
		log.Println(err)
		return
	}

	var playerToken string
	if token, ok := r.URL.Query()["token"]; ok && len(token[0]) == 32 {
		playerToken = token[0]
	}

	h.connect <- &ConnInfo{Connect: conn, Ip: getIp(r), Token: playerToken}
}

func (h *Hub) broadcast(segments ...any) {
	if h.singleplayer {
		return
	}

	h.clients.Range(func(_, v any) bool {
		client := v.(*Client)

		if !client.valid {
			return true
		}

		client.sendMsg(segments)

		return true
	})
}

func (h *Hub) processMsgs(msg *Message) []error {
	var errs []error

	if len(msg.data) < 8 || len(msg.data) > 4096 {
		return append(errs, errors.New("bad request size"))
	}

	if !verifySignature(msg.sender.key, msg.data) {
		return append(errs, errors.New("bad signature"))
	}

	if !verifyCounter(&msg.sender.counter, msg.data) {
		return append(errs, errors.New("bad counter"))
	}

	msg.data = msg.data[8:]

	for _, v := range msg.data {
		if v < 32 {
			return append(errs, errors.New("bad byte sequence"))
		}
	}

	if !utf8.Valid(msg.data) {
		return append(errs, errors.New("invalid UTF-8"))
	}

	// message processing
	for _, msgStr := range strings.Split(string(msg.data), mdelim) {
		err := h.processMsg(msgStr, msg.sender)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (h *Hub) processMsg(msgStr string, sender *Client) error {
	err := errors.New(msgStr)
	msgFields := strings.Split(msgStr, delim)

	if len(msgFields) == 0 {
		return err
	}

	if !sender.valid {
		if msgFields[0] == "ident" {
			err = h.handleIdent(msgFields, sender)
		}
	} else {
		switch msgFields[0] {
		case "m", "tp": // moved / teleported to x y
			err = h.handleM(msgFields, sender)
		case "f": // change facing direction
			err = h.handleF(msgFields, sender)
		case "spd": // change my speed to spd
			err = h.handleSpd(msgFields, sender)
		case "spr": // change my sprite
			err = h.handleSpr(msgFields, sender)
		case "fl", "rfl": // player flash / repeating player flash
			err = h.handleFl(msgFields, sender)
		case "rrfl": // remove repeating player flash
			err = h.handleRrfl(sender)
		case "h": // change sprite visibility
			err = h.handleH(msgFields, sender)
		case "sys": // change my system graphic
			err = h.handleSys(msgFields, sender)
		case "se": // play sound effect
			err = h.handleSe(msgFields, sender)
		case "ap", "mp": // add picture i/ move picture
			err = h.handleP(msgFields, sender)
		case "rp": // remove picture
			err = h.handleRp(msgFields, sender)
		case "say":
			err = h.handleSay(msgFields, sender)
		case "ss": // sync switch
			err = h.handleSs(msgFields, sender)
		case "sv": // sync variable
			err = h.handleSv(msgFields, sender)
		case "sev":
			err = h.handleSev(msgFields, sender)
		default:
			err = errors.New("unknown message type")
		}
	}

	if err != nil {
		return err
	}

	writeLog(sender.session.ip, strconv.Itoa(h.roomId), msgStr, 200)

	return nil
}

func (h *Hub) handleValidClient(client *Client) {
	if !h.singleplayer {
		// tell everyone that a new client has connected
		h.broadcast("c", client.id, client.session.uuid, client.session.rank, client.session.account, client.session.badge) // user %id% has connected message

		// send name of client
		if client.session.name != "" {
			h.broadcast("name", client.id, client.session.name)
		}

		// send the new client info about the game state
		h.clients.Range(func(_, v any) bool {
			otherClient := v.(*Client)

			if !otherClient.valid {
				return true
			}

			client.sendMsg("c", otherClient.id, otherClient.session.uuid, otherClient.session.rank, otherClient.session.account, otherClient.session.badge)
			client.sendMsg("m", otherClient.id, otherClient.x, otherClient.y)
			if otherClient.facing > 0 {
				client.sendMsg("f", otherClient.id, otherClient.facing)
			}
			client.sendMsg("spd", otherClient.id, otherClient.spd)
			if otherClient.session.name != "" {
				client.sendMsg("name", otherClient.id, otherClient.session.name)
			}
			if otherClient.session.spriteIndex >= 0 { // if the other client sent us valid sprite and index before
				client.sendMsg("spr", otherClient.id, otherClient.session.spriteName, otherClient.session.spriteIndex)
			}
			if otherClient.repeatingFlash {
				client.sendMsg("rfl", otherClient.id, otherClient.flash)
			}
			if otherClient.hidden {
				client.sendMsg("h", otherClient.id, "1")
			}
			if otherClient.session.systemName != "" {
				client.sendMsg("sys", otherClient.id, otherClient.session.systemName)
			}
			for picId, pic := range otherClient.pictures {
				client.sendMsg("ap", otherClient.id, picId, pic.positionX, pic.positionY, pic.mapX, pic.mapY, pic.panX, pic.panY, pic.magnify, pic.topTrans, pic.bottomTrans, pic.red, pic.blue, pic.green, pic.saturation, pic.effectMode, pic.effectPower, pic.name, pic.useTransparentColor, pic.fixedToMap)
			}

			return true
		})
	}

	checkHubConditions(h, client, "", "")

	for _, minigame := range h.minigameConfigs {
		score, err := getPlayerMinigameScore(client.session.uuid, minigame.MinigameId)
		if err != nil {
			writeErrLog(client.session.ip, strconv.Itoa(h.roomId), "failed to read player minigame score for "+minigame.MinigameId)
		}
		client.minigameScores = append(client.minigameScores, score)
		varSyncType := 1
		if minigame.InitialVarSync {
			varSyncType = 2
		}
		client.sendMsg("sv", minigame.VarId, varSyncType)
	}

	// send variable sync request for vending machine expeditions
	if h.roomId != currentEventVmMapId {
		return
	}

	if eventIds, hasVms := eventVms[h.roomId]; hasVms {
		for _, eventId := range eventIds {
			if eventId != currentEventVmEventId {
				continue
			}
			client.sendMsg("sev", eventId, "1")
		}
	}
}
