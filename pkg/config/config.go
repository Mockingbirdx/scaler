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
	UseFeature           bool
	IntervalInSec        map[string]int
	ColdStartNum         map[string]int
}

var DefaultConfig = Config{
	ClientAddr:           "127.0.0.1:50051",
	GcInterval:           1 * time.Millisecond,
	IdleDurationBeforeGC: 9 * time.Second,
	MaxWaitingNum:        3,
	UseFeature:           false,
	IntervalInSec: map[string]int{
		"nodes1":                      37,
		"roles1":                      300,
		"rolebindings1":               300,
		"certificatesigningrequests1": 300,
		"csinodes1":                   10,
		"nodes2":                      37,
		"roles2":                      300,
		"rolebindings2":               300,
		"certificatesigningrequests2": 300,
		"binding2":                    300,
	},
	ColdStartNum: map[string]int{
		"roles1":                      5,
		"rolebindings1":               5,
		"certificatesigningrequests1": 5,
		"roles2":                      5,
		"rolebindings2":               5,
		"certificatesigningrequests2": 5,
		"binding2":                    5,
	},
}
