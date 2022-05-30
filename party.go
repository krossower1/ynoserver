package main

import (
	"encoding/json"
	"time"

	"github.com/go-co-op/gocron"
)

type Party struct {
	Id          int           `json:"id"`
	Name        string        `json:"name"`
	Public      bool          `json:"public"`
	Pass        string        `json:"pass"`
	SystemName  string        `json:"systemName"`
	Description string        `json:"description"`
	OwnerUuid   string        `json:"ownerUuid"`
	Members     []PartyMember `json:"members"`
}

type PartyMember struct {
	Uuid          string `json:"uuid"`
	Name          string `json:"name"`
	Rank          int    `json:"rank"`
	Account       bool   `json:"account"`
	Badge         string `json:"badge"`
	SystemName    string `json:"systemName"`
	SpriteName    string `json:"spriteName"`
	SpriteIndex   int    `json:"spriteIndex"`
	MapId         string `json:"mapId,omitempty"`
	PrevMapId     string `json:"prevMapId,omitempty"`
	PrevLocations string `json:"prevLocations,omitempty"`
	X             int    `json:"x"`
	Y             int    `json:"y"`
	Online        bool   `json:"online"`
}

func startPartyUpdateTimer() {
	s := gocron.NewScheduler(time.UTC)

	s.Every(5).Seconds().Do(func() { sendPartyUpdate() })

	s.StartAsync()
}

func sendPartyUpdate() error {
	parties, err := readAllPartyData(false)
	if err != nil {
		return err //unused
	}

	for _, party := range parties { //for every party
		var partyPass string
		if party.Pass != "" {
			partyPass = party.Pass
			party.Pass = ""
		}
		partyDataJson, err := json.Marshal(party)
		if err != nil {
			continue
		}

		for _, member := range party.Members { //for every member
			if member.Online {
				if client, ok := sessionClients[member.Uuid]; ok {
					var jsonData string
					if member.Uuid == party.OwnerUuid {
						// Expose password only for party owner
						party.Pass = partyPass
						ownerPartyDataJson, err := json.Marshal(party)
						party.Pass = ""
						if err != nil {
							continue
						}
						jsonData = string(ownerPartyDataJson)
					} else {
						jsonData = string(partyDataJson)
					}
					client.send <- []byte("pt" + paramDelimStr + jsonData) //send JSON to client
				}
			}
		}
	}

	return nil
}
