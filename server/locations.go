/*
	Copyright (C) 2021-2024  The YNOproject Developers

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

package server

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type GameLocation struct {
	Id     int      `json:"id"`
	Game   string   `json:"game"`
	Name   string   `json:"name"`
	MapIds []string `json:"mapIds"`
}

type PathLocations struct {
	Locations []PathLocation `json:"locations"`
}

type PathLocation struct {
	Title      string                    `json:"title"`
	TitleJP    string                    `json:"titleJP"`
	ConnType   int                       `json:"connType"`
	TypeParams map[string]ConnTypeParams `json:"typeParams"`
	Depth      int                       `json:"depth"`
}

type ConnTypeParams struct {
	Params   string `json:"params"`
	ParamsJP string `json:"paramsJP"`
}

func getNext2kkiLocations(originLocationName string, destLocationName string) (PathLocations, error) {
	var nextLocations PathLocations

	response, err := query2kki("getNextLocations", fmt.Sprintf("origin=%s&dest=%s", url.QueryEscape(originLocationName), url.QueryEscape(destLocationName)))
	if err != nil {
		return nextLocations, err
	}

	err = json.Unmarshal([]byte(response), &nextLocations.Locations)
	if err != nil {
		return nextLocations, err
	}

	return nextLocations, nil
}
