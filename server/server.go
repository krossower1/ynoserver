// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gorilla/websocket"
)

var (
	maxID = 512
	totalPlayerCount = 0
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	isOkName = regexp.MustCompile("^[A-Za-z0-9]+$").MatchString
	paramDelimStr = "\uffff"
	msgDelimStr = "\ufffe"

	config Config
	db *sql.DB
)

type ConnInfo struct {
	Connect *websocket.Conn
	Ip string
}

func writeLog(ip string, roomName string, payload string, errorcode int) {
	log.Printf("%v %v \"%v\" %v\n", ip, roomName, strings.Replace(payload, "\"", "'", -1), errorcode)
}

func writeErrLog(ip string, roomName string, payload string) {
	writeLog(ip, roomName, payload, 400)
}

func CreateAllHubs(roomNames []string) {
	h := HubController{}

	db = getDatabaseHandle()

	for _, roomName := range roomNames {
		h.addHub(roomName)
	}
}

func NewHub(roomName string, h *HubController) *Hub {
	return &Hub{
		processMsgCh:  make(chan *Message),
		connect:   make(chan *ConnInfo),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		id: make(map[int]bool),
		roomName: roomName,
		controller: h,
	}
}

type IpHubResponse struct {
	IP          string `json:"ip"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
	Asn         int    `json:"asn"`
	Isp         string `json:"isp"`
	Block       int    `json:"block"`
}

func isVpn(ip string) (bool, error) {
	apiKey := ""

	if apiKey == "" {
		return false, nil //VPN checking is not available
	}

	req, err := http.NewRequest("GET", "http://v2.api.iphub.info/ip/" + ip, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("X-Key", apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var response IpHubResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return false, err
	}

	var blockedIp bool
	if response.Block == 0 {
		blockedIp = false
	} else {
		blockedIp = true
	}
	
	if response.Block > 0 {
		log.Printf("Connection Blocked %v %v %v %v\n", response.IP, response.CountryName, response.Isp, response.Block)
		return false, errors.New("connection banned")
	}

	return blockedIp, nil
}

func isValidSpriteName(name string) bool {
	if name == "" {
		return true
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return false
	}

	for _, otherName := range config.spriteNames {
		if strings.EqualFold(otherName, name) {
			return true
		}
	}
	return false
}

func isValidSystemName(name string) bool {
	for _, otherName := range config.systemNames {
		if strings.EqualFold(otherName, name) {
			return true
		}
	}
	return false
}

func isValidSoundName(name string) bool {
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return false
	}

	for _, otherName := range config.soundNames {
		if strings.EqualFold(otherName, name) {
			for _, ignoredName := range config.ignoredSoundNames {
				if strings.EqualFold(ignoredName, name) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func isValidPicName(name string) bool {
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return false
	}

	nameLower := strings.ToLower(name)
	for _, otherName := range config.pictureNames {
		if otherName == nameLower {
			return true
		}
	}
	for _, prefix := range config.picturePrefixes {
		if strings.HasPrefix(nameLower, prefix) {
			return true
		}
	}

	return false
}
