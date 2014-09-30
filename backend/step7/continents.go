package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"appengine"
	"appengine/memcache"
	"appengine/urlfetch"
)

const freebaseKey = "get your own key from https://developers.google.com/freebase/v1/getting-started"

// these countries are special cases not handled correctly with the current freebase query.
var forcedContinents = map[string]string{
	"US": "North America", // instead of Americas
	"RU": "Europe, Asia ", // instead of Eurasia
	"NL": "Europe",        // NL is contained by the Kingdom of Netherlands, which is in Europe
	"TR": "Europe, Asia",  // because nobody knows
}

type location struct {
	Type string      `json:"type"`
	Name MaybeString `json:"name"`
}

type country struct {
	Type        string     `json:"type"`
	CountryCode string     `json:"iso3166_1_alpha2"`
	ContainedBy []location `json:"/location/location/containedby"`
}

func continent(c appengine.Context, countryCode string) (string, error) {
	// first check if it's a special country.
	if c, ok := forcedContinents[countryCode]; ok {
		return c, nil
	}

	// fetch from memcache.
	key := "cc:" + strings.ToLower(countryCode)
	item, err := memcache.Get(c, key)
	if err == nil {
		return string(item.Value), nil
	}
	if err != memcache.ErrCacheMiss {
		c.Errorf("memcache get %q: %v", key, err)
	}

	// prepare freebase query
	data := []country{{
		Type:        "/location/country",
		CountryCode: countryCode,
		ContainedBy: []location{{Type: "/location/continent"}},
	}}

	// encode the query data to json.
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(data); err != nil {
		return "", fmt.Errorf("encode freebase query: %v", err)
	}

	// create a url containing the url escaped freebase query.
	u := url.URL{
		Scheme:   "https",
		Host:     "www.googleapis.com",
		Path:     "/freebase/v1/mqlread",
		RawQuery: "key=" + freebaseKey + "&query=",
	}
	u.RawQuery += url.QueryEscape(buf.String())

	// send the http request and check everything worked correctly.
	res, err := urlfetch.Client(c).Get(u.String())
	if err != nil {
		return "", fmt.Errorf("get freebase: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get freebase: %s", res.Status)
	}

	// use the same structure we used to prepare the query to parse the result.
	result := struct {
		Result []country `json:"result"`
	}{data}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		log.Fatalf("decode body: %v", err)
	}
	if len(result.Result) == 0 {
		return "", fmt.Errorf("cannot find country with code %v", countryCode)
	}

	// find the best continent name (prefer North/South America to Americas)
	cons := result.Result[0].ContainedBy
	if len(cons) == 0 {
		return "", fmt.Errorf("cannot find continent for %v", countryCode)
	}
	loc := cons[0].Name.String()
	for len(cons) > 1 && loc == "Americas" {
		loc = cons[1].Name.String()
	}

	// store to memcache for a long period.
	err = memcache.Set(c, &memcache.Item{
		Key:        key,
		Value:      []byte(loc),
		Expiration: 30 * 24 * time.Hour,
	})
	if err != nil {
		c.Errorf("memcache set: %v", err)
	}
	return loc, nil
}

// MaybeString satisfies json.Marshaler and json.Unmarshaler printing
// null when no value has been set to the internal string.
// This is the expected behavior for the Freebase API.
type MaybeString struct{ s *string }

// MaybeString satisfies fmt.Stringer.
func (m MaybeString) String() string {
	if m.s == nil {
		return "null"
	}
	return *m.s
}

// MaybeString satisfies json.Marhsaler.
func (m MaybeString) MarshalJSON() ([]byte, error) {
	if m.s == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*m.s)
}

// MaybeString satisfies json.Unmarhsaler.
func (m *MaybeString) UnmarshalJSON(b []byte) error {
	if string(b) != "null" {
		m.s = new(string)
		return json.Unmarshal(b, m.s)
	}
	return nil
}
