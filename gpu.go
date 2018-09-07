package main

import (
	"io/ioutil"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"log"
	"strings"
)

const (
	nvRegex = `nvidia(?P<devIdr>\d+)`
)

func check(err error) {
	if err != nil {
		log.Panicln("Fatal:", err)
	}
}

func getDevices() (devs []*pluginapi.Device) {
	files, err := ioutil.ReadDir("/dev/")
	if err != nil {
		return
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "nvidia") {
			params := getParams(nvRegex, f.Name())
			devId, ok := params["devId"]
			if !ok {
				//log.Printf("File '%s' does not match '%s'", f.Name(), nvRegex)
				continue
			}
			//log.Printf("Add Device: %s", devId)
			devs = append(devs, &pluginapi.Device{ID: devId, Health: "healthy"})
		}
	}
	devs = append(devs, &pluginapi.Device{
			ID:     "0",
			Health: pluginapi.Healthy,
		})
	return devs
}

func deviceExists(devs []*pluginapi.Device, id string) bool {
	for _, d := range devs {
		if d.ID == id {
			return true
		}
	}
	return false
}

