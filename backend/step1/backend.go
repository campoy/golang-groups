/*
 Copyright 2011 The Go Authors.  All rights reserved.
 Use of this source code is governed by a BSD-style
 license that can be found in the LICENSE file.
*/

package backend

import (
	"encoding/json"
	"net/http"

	"appengine"
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

// What it the information changes?
var groups = []Group{
	{"GoSV", "http://www.meetup.com/golangsv", 194, "San Mateo", "US"},
	{"GoSF", "http: //www.meetup.com/golangsf", 1393, "San Francisco", "US"},
}

func getGroups(w http.ResponseWriter, r *http.Request) {
	// first we build the response
	res := struct {
		Groups []Group
		Errors []string
	}{
		groups,
		[]string{"something bad happened"},
	}

	// then we encode it as JSON on the response
	enc := json.NewEncoder(w)
	err := enc.Encode(res)

	// And if encoding fails we log the error
	if err != nil {
		c := appengine.NewContext(r)
		c.Errorf("encode response: %v", err)
	}
}
