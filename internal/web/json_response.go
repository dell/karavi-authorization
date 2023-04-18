// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// JSONError wraps a json error response
type JSONError struct {
	ErrorMsg string `json:"error"`
	Code     int    `json:"code"`
}

func (e JSONError) Error() string {
	return e.ErrorMsg
}

// JSONErrorResponse writes an error to an http ResponseWriter
func JSONErrorResponse(w http.ResponseWriter, code int, err error) error {
	b, err := json.Marshal(&JSONError{ErrorMsg: err.Error(), Code: code})
	if err != nil {
		return err
	}
	w.WriteHeader(code)
	_, err = w.Write(b)
	if err != nil {
		log.Println("Failed to write json error response", err)
	}
	return nil
}

// PowerScaleAPIError is the error format returned from PowerScale
type PowerScaleAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PowerScaleJSONErrorResponse writes a PowerScaleAPIError response
func PowerScaleJSONErrorResponse(w http.ResponseWriter, code int, psErr error) error {
	errBody := struct {
		Err []PowerScaleAPIError `json:"errors"`
	}{
		Err: []PowerScaleAPIError{
			{
				Code:    strconv.Itoa(code),
				Message: psErr.Error(),
			},
		},
	}
	err := json.NewEncoder(w).Encode(&errBody)
	if err != nil {
		log.Println("Failed to encode error response", err)
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
	return nil
}
