/*
 Copyright 2011 The Go Authors.  All rights reserved.
 Use of this source code is governed by a BSD-style
 license that can be found in the LICENSE file.
*/

package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"appengine"
	"appengine/urlfetch"
)

func init() {
	http.HandleFunc("/api/groups", getGroups)
}

type Group struct {
	Name    string
	URL     string
	Members int
	City    string
	Country string
}

var ids = []string{"golangsf", "golangsv"}

func getGroups(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	var res struct {
		Groups []*Group
		Errors []string
	}

	// let's fetch every group
	for _, id := range ids {
		group, err := fetch(c, id)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("fetch %v: %v", id, err))
			continue
		}
		res.Groups = append(res.Groups, group)
	}

	// then we encode it as JSON on the response
	enc := json.NewEncoder(w)
	err := enc.Encode(res)

	// And if encoding fails we log the error
	if err != nil {
		c.Errorf("encode response: %v", err)
	}
}

// fetch fetches a meetup group given its id from using the meetup API
// docs for the API: http://www.meetup.com/meetup_api/docs/
func fetch(c appengine.Context, id string) (*Group, error) {
	const (
		apiKey      = "13755230371f55176d32a31147e2614"
		urlTemplate = "https://api.meetup.com/%s?sign=true&key=%s"
	)

	client := urlfetch.Client(c)
	res, err := client.Get(fmt.Sprintf(urlTemplate, id, apiKey))
	if err != nil {
		return nil, fmt.Errorf("get: %v", err)
	}

	var g struct {
		Name    string `json:"name"`
		Link    string `json:"link"`
		City    string `json:"city"`
		Country string `json:"country"`
		Members int    `json:"members"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	dec := json.NewDecoder(res.Body)
	err = dec.Decode(&g)
	if err != nil {
		return nil, fmt.Errorf("decode: %v", err)
	}

	if len(g.Errors) > 0 {
		var errs []string
		for _, e := range g.Errors {
			errs = append(errs, e.Message)
		}
		return nil, fmt.Errorf(strings.Join(errs, "\n"))
	}

	return &Group{
		Name:    g.Name,
		URL:     g.Link,
		Members: g.Members,
		City:    g.City,
		Country: g.Country,
	}, nil

}
