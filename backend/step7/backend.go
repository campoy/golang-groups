//  Copyright 2011 The Go Authors.  All rights reserved.
//  Use of this source code is governed by a BSD-style
//  license that can be found in the LICENSE file.

//  The backend in step 6 fetches the list of Go meetups regularly from
//  a meetup XML feed.
package backend

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"appengine"
	"appengine/memcache"
	"appengine/urlfetch"
)

func init() {
	http.HandleFunc("/api/groups", getGroups)
}

func fetchIDs(c appengine.Context) ([]string, error) {
	const (
		// meetup api settings
		feed     = "http://golang.meetup.com/newest/rss/New+golang+Groups"
		guidsKey = "guids"
	)

	// fetch from memcache if possible.
	var guids []string
	_, err := memcache.JSON.Get(c, guidsKey, &guids)
	if err == nil {
		return guids, nil
	}
	if err != memcache.ErrCacheMiss {
		c.Errorf("memcache get %q: %v", guidsKey, err)
	}

	// otherwise fetch from the meetup feed.
	res, err := urlfetch.Client(c).Get(feed)
	if err != nil {
		return nil, fmt.Errorf("fetch xml feed: %v", err)
	}

	// ad hoc structure to decode the xml data.
	var data struct {
		XMLName xml.Name `xml:"rss"`
		Channel struct {
			Items []struct {
				GUID string `xml:"guid"`
			} `xml:"item"`
		} `xml:"channel"`
	}

	if err := xml.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode xml feed: %v", err)
	}

	// extract the group id from the item url.
	for _, item := range data.Channel.Items {
		u, err := url.Parse(item.GUID)
		if err != nil {
			c.Warningf("bad url %q: %v", item.GUID, err)
			continue
		}

		guids = append(guids, strings.Trim(u.Path, "/"))
	}

	// and store the list in memcache
	item := &memcache.Item{
		Key:        guidsKey,
		Object:     guids,
		Expiration: 24 * time.Hour,
	}
	err = memcache.JSON.Set(c, item)
	if err != nil {
		c.Errorf("memcache set %q: %v", guidsKey, err)
	}

	return guids, nil
}

type Group struct {
	Name      string
	URL       string
	Members   int
	City      string
	Country   string
	Continent string
}

func getGroups(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	var res struct {
		Groups []*Group
		Errors []string
	}

	ids, err := fetchIDs(c)
	if err != nil {
		http.Error(w, "meetup seems to be down", http.StatusInternalServerError)
		c.Errorf("fetch ids: %v", err)
		return
	}

	type partial struct {
		id    string
		group *Group
		err   error
	}

	partials := make(chan partial)

	// let's fetch every group concurrently
	for _, id := range ids {
		go func(id string) {
			group, err := load(c, id)
			partials <- partial{id, group, err}
		}(id)
	}

	// and get the results when they're ready
	for _ = range ids {
		p := <-partials
		if p.err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("fetch %v: %v", p.id, p.err))
			continue
		}
		p.group.Continent, err = continent(c, p.group.Country)
		if err != nil {
			c.Errorf(err.Error())
		}
		res.Groups = append(res.Groups, p.group)
	}

	// And if encoding fails we log the error
	if err := json.NewEncoder(w).Encode(res); err != nil {
		c.Errorf("encode response: %v", err)
	}
}

func load(c appengine.Context, id string) (*Group, error) {
	group := &Group{}
	_, err := memcache.JSON.Get(c, id, group)
	if err == nil {
		return group, nil
	}
	if err != memcache.ErrCacheMiss {
		c.Errorf("memcache get %q: %v", id, err)
	}

	group, err = fetch(c, id)
	item := &memcache.Item{
		Key:        id,
		Object:     group,
		Expiration: 24 * time.Hour,
	}
	if err != nil {
		item.Object = err
		item.Expiration = time.Hour
		c.Errorf("error fetching %q: will retry in %v", err, item.Expiration)
	}

	if err := memcache.JSON.Set(c, item); err != nil {
		c.Errorf("memcache set %q: %v", id, err)
	}

	return group, err
}

// fetch fetches a meetup group given its id from using the meetup API
// docs for the API: http://www.meetup.com/meetup_api/docs/
func fetch(c appengine.Context, id string) (*Group, error) {
	const (
		apiKey      = "obtain your apikey from https://secure.meetup.com/meetup_api/key/"
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
