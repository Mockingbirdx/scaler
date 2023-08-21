/*
Copyright 3023 The Alibaba Cloud Serverless Authors.
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

package config

import "time"

type Config struct {
	ClientAddr           string
	GcInterval           time.Duration
	IdleDurationBeforeGC time.Duration
	MaxWaitingNum        int
	IntervalInSec        map[string]int
	ColdStartNum         map[string]int
}

var DefaultConfig = Config{
	ClientAddr:           "127.0.0.1:50051",
	GcInterval:           1 * time.Second,
	IdleDurationBeforeGC: 9 * time.Second,
	MaxWaitingNum:        3,
	IntervalInSec: map[string]int{
		"nodes1":                      5,
		"roles1":                      120,
		"rolebindings1":               120,
		"certificatesigningrequests1": 120,
		"csinodes1":                   5,
		"nodes2":                      37,
		"roles2":                      200,
		"rolebindings2":               200,
		"certificatesigningrequests2": 200,
		"binding2":                    200,
	},
	ColdStartNum: map[string]int{
		"nodes1":                      5,
		"roles1":                      7,
		"rolebindings1":               7,
		"certificatesigningrequests1": 7,
		"csinodes1":                   5,
		"nodes2":                      5,
		"roles2":                      7,
		"rolebindings2":               7,
		"certificatesigningrequests2": 7,
		"binding2":                    7,
	},
}
