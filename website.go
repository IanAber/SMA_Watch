package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func setUpWebSite(port int) {

	router := mux.NewRouter().StrictSlash(true)
	// Return the current values as a JSON object
	router.HandleFunc("/", getValues).Methods("GET")

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), router))
}

func getValues(w http.ResponseWriter, _ *http.Request) {

	var total float64

	panels.mu.Lock()
	defer panels.mu.Unlock()
	for _, s := range panels.InverterStrings {
		total += s.Power
	}
	panels.TotalPower = total
	if body, err := json.Marshal(&panels); err != nil {
		ReturnJSONError(w, "Solar Panels", err, http.StatusInternalServerError, true)
	} else {
		if _, err := fmt.Fprintf(w, string(body)); err != nil {
			log.Println(err)
		}
	}
}
