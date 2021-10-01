/*
   Copyright 2021 Splunk Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package provider

import (
	"encoding/json"
	"log"
	"net/http"
)

func debug(msg interface{}) {
	log.Println("########## STACKROX ##########:", msg)
}

func logMessage(message interface{}) {
	data, err := json.MarshalIndent(message, "", "    ")
	if err != nil {
		panic(err)
	}
	debug(string(data))
}

func logResult(result interface{}, resp *http.Response, err error) {
	debug(err)
	debug(resp)
	debug(result)
}
