package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

const (
	bucketMetadata      = "metadata"
	bucketNotifications = "notifications"

	keyGenerationID = "generationID"
)

// A NotificationsResponse contains a sequence of notifications for a given generation ID.
type NotificationsResponse struct {
	GenerationID  string         `json:"generationID"`
	Notifications []Notification `json:"notifications"`
}

// A Notification models a notification with its data and a sequential index that is valid
// within a given generation ID.
type Notification struct {
	Index uint64     `json:"index"`
	Data  JSONString `json:"data"`
}

// A JSONString is a string that gets marshalled verbatim into JSON,
// as it is expected to already contain valid JSON.
type JSONString string

// MarshalJSON implements json.Marshaler.
func (js JSONString) MarshalJSON() ([]byte, error) {
	return []byte(js), nil
}

func serve(addr string, store notificationStore) error {
	http.HandleFunc("/append", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, fmt.Sprintf("invalid method %s", r.Method), http.StatusBadRequest)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = json.Unmarshal(body, &map[string]interface{}{}); err != nil {
			http.Error(w, fmt.Sprintf("body is not a valid JSON object: %v", err), http.StatusBadRequest)
			return
		}

		if err = store.append(body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, fmt.Sprintf("invalid method %s", r.Method), http.StatusBadRequest)
			return
		}

		genID := r.FormValue("generationID")
		fromIdx := r.FormValue("fromIndex")

		if fromIdx == "" {
			fromIdx = "0"
		}

		idx, err := strconv.ParseUint(fromIdx, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid 'fromIndex': %v", err), http.StatusBadRequest)
			return
		}

		notifications, err := store.get(genID, idx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		marshalled, err := json.Marshal(notifications)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if _, err := w.Write(marshalled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	return http.ListenAndServe(addr, nil)
}

func main() {
	storagePath := flag.String("storage-path", "notifications.db", "The path for storing notification data.")
	listenAddr := flag.String("listen-address", ":9099", "The address to listen on for web requests.")
	flag.Parse()

	store, err := newBoltStore(*storagePath)
	if err != nil {
		log.Fatalln("Error opening notification store:", err)
	}
	defer store.close()

	log.Printf("Listening on %v...", *listenAddr)
	log.Fatalln(serve(*listenAddr, store))
}
