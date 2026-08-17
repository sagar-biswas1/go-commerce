package utils

import (
	"encoding/json"
	"log"
	"net/http"
)

func SendData (w http.ResponseWriter, data interface{}, statusCode int){
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		encoder:=json.NewEncoder(w)
		if err:= encoder.Encode(data); err != nil {
			// The status line is already sent, so the client only sees a short body.
			log.Printf("encode response: %v", err)
		}
}

// SendError replies with a JSON body so success and failure share one shape.
func SendError (w http.ResponseWriter, message string, statusCode int){
		SendData(w, map[string]string{"error": message}, statusCode)
}
