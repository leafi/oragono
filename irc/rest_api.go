// Copyright (c) 2016- Daniel Oaks <daniel@danieloaks.net>
// released under the MIT license

// viewing and modifying accounts, registered channels, dlines, rehashing, etc

package irc

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"fmt"

	"github.com/gorilla/mux"
	"github.com/tidwall/buntdb"
)

const restErr = "{\"error\":\"An unknown error occurred\"}"

// restAPIServer is used to keep a link to the current running server since this is the best
// way to do it, given how HTTP handlers dispatch and work.
var restAPIServer *Server

type restInfoResp struct {
	ServerName  string `json:"server-name"`
	NetworkName string `json:"network-name"`

	Version string `json:"version"`
}

type restStatusResp struct {
	Clients  int `json:"clients"`
	Opers    int `json:"opers"`
	Channels int `json:"channels"`
}

type restDLinesResp struct {
	DLines map[string]IPBanInfo `json:"dlines"`
}

type restAcct struct {
	Name         string    `json:"name"`
	RegisteredAt time.Time `json:"registered-at"`
	Clients      int       `json:"clients"`
}

type restAccountsResp struct {
	Verified map[string]restAcct `json:"verified"`
}

type restRehashResp struct {
	Successful bool      `json:"successful"`
	Error      string    `json:"error"`
	Time       time.Time `json:"time"`
}

func restInfo(w http.ResponseWriter, r *http.Request) {
	rs := restInfoResp{
		Version:     SemVer,
		ServerName:  restAPIServer.name,
		NetworkName: restAPIServer.networkName,
	}
	b, err := json.Marshal(rs)
	if err != nil {
		fmt.Fprintln(w, restErr)
	} else {
		fmt.Fprintln(w, string(b))
	}
}

func restStatus(w http.ResponseWriter, r *http.Request) {
	rs := restStatusResp{
		Clients:  restAPIServer.clients.Count(),
		Opers:    len(restAPIServer.operators),
		Channels: len(restAPIServer.channels),
	}
	b, err := json.Marshal(rs)
	if err != nil {
		fmt.Fprintln(w, restErr)
	} else {
		fmt.Fprintln(w, string(b))
	}
}

func restGetDLines(w http.ResponseWriter, r *http.Request) {
	rs := restDLinesResp{
		DLines: restAPIServer.dlines.AllBans(),
	}
	b, err := json.Marshal(rs)
	if err != nil {
		fmt.Fprintln(w, restErr)
	} else {
		fmt.Fprintln(w, string(b))
	}
}

func restGetAccounts(w http.ResponseWriter, r *http.Request) {
	rs := restAccountsResp{
		Verified: make(map[string]restAcct),
	}

	// get accounts
	err := restAPIServer.store.View(func(tx *buntdb.Tx) error {
		tx.AscendKeys("account.exists *", func(key, value string) bool {
			key = key[len("account.exists "):]
			_, err := tx.Get(fmt.Sprintf(keyAccountVerified, key))
			verified := err == nil
			fmt.Println(fmt.Sprintf(keyAccountVerified, key))

			// get other details
			name, _ := tx.Get(fmt.Sprintf(keyAccountName, key))
			regTimeStr, _ := tx.Get(fmt.Sprintf(keyAccountRegTime, key))
			regTimeInt, _ := strconv.ParseInt(regTimeStr, 10, 64)
			regTime := time.Unix(regTimeInt, 0)

			var clients int
			acct := restAPIServer.accounts[key]
			if acct != nil {
				clients = len(acct.Clients)
			}

			if verified {
				rs.Verified[key] = restAcct{
					Name:         name,
					RegisteredAt: regTime,
					Clients:      clients,
				}
			} else {
				//TODO(dan): Add to unverified list
			}

			return true // true to continue I guess?
		})

		return nil
	})

	b, err := json.Marshal(rs)
	if err != nil {
		fmt.Fprintln(w, restErr)
	} else {
		fmt.Fprintln(w, string(b))
	}
}

func restRehash(w http.ResponseWriter, r *http.Request) {
	err := restAPIServer.rehash()

	rs := restRehashResp{
		Successful: err == nil,
		Time:       time.Now(),
	}
	if err != nil {
		rs.Error = err.Error()
	}

	b, err := json.Marshal(rs)
	if err != nil {
		fmt.Fprintln(w, restErr)
	} else {
		fmt.Fprintln(w, string(b))
	}
}

func (s *Server) startRestAPI() {
	// so handlers can ref it later
	restAPIServer = s

	// start router
	r := mux.NewRouter()

	// GET methods
	rg := r.Methods("GET").Subrouter()
	rg.HandleFunc("/info", restInfo)
	rg.HandleFunc("/status", restStatus)
	rg.HandleFunc("/dlines", restGetDLines)
	rg.HandleFunc("/accounts", restGetAccounts)

	// PUT methods
	rp := r.Methods("POST").Subrouter()
	rp.HandleFunc("/rehash", restRehash)

	// start api
	go http.ListenAndServe(s.restAPI.Listen, r)
}
