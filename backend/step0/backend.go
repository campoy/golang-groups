//  Copyright 2011 The Go Authors.  All rights reserved.
//  Use of this source code is governed by a BSD-style
//  license that can be found in the LICENSE file.

//  The backend in step 0 prints the same constant string as the response to
//  every HTTP request.
package backend

import (
	"fmt"
	"net/http"
)

func init() {
	http.HandleFunc("/api/groups", getGroups)
}

// Not very dynamic ... and how do we know if this is valid JSON?
const response = `
    {
        "Groups": [{
            "Name": "GoSV",
            "URL": "http://www.meetup.com/golangsv",
            "Members": 194,
            "City": "San Mateo",
            "Country": "US"
        }, {
            "Name": "GoSF",
            "URL": "http: //www.meetup.com/golangsf",
            "Members": 1393,
            "City": "San Francisco",
            "Country": "US"
        }],
        "Errors": [
            "something bad happened"
        ]
    }
`

func getGroups(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, response)
}
