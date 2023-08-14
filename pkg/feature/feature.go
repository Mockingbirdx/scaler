package feature

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
)

type AppFeature struct {
	Key              string `json:"Key"`
	ExecDurationInMs int32  `json:"ExecDurationInMs"`
	InitDurationInMs int32  `json:"InitDurationInMs"`
	CycleInSec       int32  `json:"CycleInSec"`
	TermInSec        int32  `json:"TermInSec"`
}

var AppFeatures = make(map[string]*AppFeature)

func ReadFeature() {
	filename := "/app/source/pkg/feature/feature.json"
	f, e := os.Open(filename)
	if e != nil {
		log.Println("Open file error: " + e.Error())
	}

	buf := bufio.NewScanner(f)
	for {
		if !buf.Scan() {
			break
		}
		line := buf.Text()

		var app AppFeature
		json.Unmarshal([]byte(line), &app)
		AppFeatures[app.Key] = &app
	}

	if err := f.Close(); err != nil {
		log.Println("Close file error: " + e.Error())
	}
	log.Printf("ReadFeature: AppFeatures len = %d", len(AppFeatures))
}
