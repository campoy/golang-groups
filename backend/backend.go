//  Copyright 2011 The Go Authors.  All rights reserved.
//  Use of this source code is governed by a BSD-style
//  license that can be found in the LICENSE file.

//  The backend in step 6 fetches the list of Go meetups regularly from
//  a meetup XML feed.
package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"appengine"
	"appengine/memcache"
	"appengine/urlfetch"
)

var apiKey string

func init() {
	apiKey = os.Getenv("API_KEY")
	if apiKey == "" {
		panic("missing meetup api key")
	}
	http.HandleFunc("/api/groups", getGroups)
}

// list of fields to fetch from the API, keep in sync with the struct below.
var fields = []string{"name", "link", "members", "city", "lat", "lon", "country"}

type Group struct {
	Name     string
	Link     string
	Members  int
	City     string
	Lat, Lon float64
	Country  string
}

func getGroups(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	var res struct {
		Groups []Group `json:",omitempty"`
		Error  string  `json:",omitempty"`
	}

	gs, err := fetchAll(c, "golang", "go-programming-language")
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Groups = gs
	}

	b, err := json.MarshalIndent(res, "", "\t")
	if err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
		c.Errorf("could not encode response: %v", err)
		return
	}
	fmt.Fprintf(w, "%s", b)
}

func fetchAll(c appengine.Context, topics ...string) ([]Group, error) {
	var groups []Group
	if _, err := memcache.JSON.Get(c, "groups", &groups); err == nil {
		return groups, nil
	}

	const feedTmpl = "https://api.meetup.com/2/groups?sign=true&key=%s&topic=%s&only=%s"

	byURL := map[string]Group{}
	client := urlfetch.Client(c)

	for _, topic := range topics {
		next := fmt.Sprintf(feedTmpl, apiKey, topic, strings.Join(fields, ","))
		for next != "" {
			res, err := client.Get(next)
			if err != nil {
				return nil, fmt.Errorf("could not get groups by topic ID: %v", err)
			}
			defer res.Body.Close()

			var data struct {
				Results []Group
				Problem string
				Details string
				Meta    struct {
					Next string
				}
			}

			if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
				return nil, fmt.Errorf("could not decode the Meetup API response: %v", err)
			}
			if data.Problem != "" || data.Details != "" {
				return nil, fmt.Errorf("%s %s", data.Problem, data.Details)
			}

			for _, g := range data.Results {
				byURL[g.Link] = g
			}
			next = data.Meta.Next
		}
	}

	for _, g := range byURL {
		groups = append(groups, g)
	}

	err := memcache.JSON.Set(c, &memcache.Item{
		Key:        "groups",
		Object:     groups,
		Expiration: 5 * time.Minute,
	})
	if err != nil {
		c.Errorf("memcache set: %v", err)
	}

	return groups, nil
}
