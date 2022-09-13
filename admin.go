package main

import (
	"encoding/json"
	"net/http"
)

func adminGetOnlinePlayers(w http.ResponseWriter, r *http.Request) {
	_, _, rank, _, _, _ := getPlayerDataFromToken(r.Header.Get("Authorization"))
	if rank < 1 {
		handleError(w, r, "access denied")
		return
	}

	var response []PlayerInfo

	for _, client := range sessionClients {
		player := PlayerInfo{
			Uuid: client.uuid,
			Name: client.name,
			Rank: client.rank,
		}

		response = append(response, player)
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		handleError(w, r, "error while marshaling")
	}

	w.Write(responseJson)
}

func adminGetBans(w http.ResponseWriter, r *http.Request) {
	_, _, rank, _, _, _ := getPlayerDataFromToken(r.Header.Get("Authorization"))
	if rank < 1 {
		handleError(w, r, "access denied")
		return
	}

	responseJson, err := json.Marshal(getBannedPlayers())
	if err != nil {
		handleError(w, r, "error while marshaling")
	}

	w.Write(responseJson)
}

func adminGetMutes(w http.ResponseWriter, r *http.Request) {
	_, _, rank, _, _, _ := getPlayerDataFromToken(r.Header.Get("Authorization"))
	if rank < 1 {
		handleError(w, r, "access denied")
		return
	}

	responseJson, err := json.Marshal(getMutedPlayers())
	if err != nil {
		handleError(w, r, "error while marshaling")
	}

	w.Write(responseJson)
}

func adminBan(w http.ResponseWriter, r *http.Request) {
	uuid, _, rank, _, _, _ := getPlayerDataFromToken(r.Header.Get("Authorization"))
	if rank < 1 {
		handleError(w, r, "access denied")
		return
	}

	playerParam, ok := r.URL.Query()["player"]
	if !ok || len(playerParam) < 1 {
		handleError(w, r, "player not specified")
		return
	}

	err := tryBanPlayer(uuid, playerParam[0])
	if err != nil {
		handleInternalError(w, r, err)
		return
	}

	w.Write([]byte("ok"))
}

func adminMute(w http.ResponseWriter, r *http.Request) {
	uuid, _, rank, _, _, _ := getPlayerDataFromToken(r.Header.Get("Authorization"))
	if rank < 1 {
		handleError(w, r, "access denied")
		return
	}

	playerParam, ok := r.URL.Query()["player"]
	if !ok || len(playerParam) < 1 {
		handleError(w, r, "player not specified")
		return
	}

	err := tryMutePlayer(uuid, playerParam[0])
	if err != nil {
		handleInternalError(w, r, err)
		return
	}

	w.Write([]byte("ok"))
}

func adminUnban(w http.ResponseWriter, r *http.Request) {
	uuid, _, rank, _, _, _ := getPlayerDataFromToken(r.Header.Get("Authorization"))
	if rank < 1 {
		handleError(w, r, "access denied")
		return
	}

	playerParam, ok := r.URL.Query()["player"]
	if !ok || len(playerParam) < 1 {
		handleError(w, r, "player not specified")
		return
	}

	err := tryUnbanPlayer(uuid, playerParam[0])
	if err != nil {
		handleInternalError(w, r, err)
		return
	}

	w.Write([]byte("ok"))
}

func adminUnmute(w http.ResponseWriter, r *http.Request) {
	uuid, _, rank, _, _, _ := getPlayerDataFromToken(r.Header.Get("Authorization"))
	if rank < 1 {
		handleError(w, r, "access denied")
		return
	}

	playerParam, ok := r.URL.Query()["player"]
	if !ok || len(playerParam) < 1 {
		handleError(w, r, "player not specified")
		return
	}

	err := tryUnmutePlayer(uuid, playerParam[0])
	if err != nil {
		handleInternalError(w, r, err)
		return
	}

	w.Write([]byte("ok"))
}
