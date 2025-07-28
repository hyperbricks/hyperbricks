package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

func handler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	ServeContent(w, r)

	elapsed := time.Since(start)

	// counter logic (temporary...)
	requestCounterMutex.Lock()
	requestCounter = requestCounter + 1
	counter := requestCounter
	requestCounterMutex.Unlock()

	if counter%100 == 0 {
		fmt.Println("Requests processed", counter)
	}

	logging.GetLogger().Debugw("Request processed", "duration", elapsed)
}
