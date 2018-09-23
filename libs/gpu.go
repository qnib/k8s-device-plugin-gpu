package qniblib // import "github.com/qnib/k8s-device-plugin-gpu/libs"

import (
	"github.com/zpatrick/go-config"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"log"
	"strings"
)

const (
	nvRegex = `/dev/nvidia(?P<devId>\d+)`
)

func check(err error) {
	if err != nil {
		log.Panicln("Fatal:", err)
	}
}

func GetDevices(cfg *config.Config) (devices []*pluginapi.Device) {
	key := "devices.gpus"
	val, err := cfg.String(key)
	if err != nil {
		log.Fatalf("No key '%s' holdinfg a list of GPUs", key)
	}
	devs := strings.Split(val, ",")
	for _, dev := range devs {
		params := getParams(nvRegex, dev)
		devId, ok := params["devId"]
		if !ok {
		    log.Printf("Path '%s' does not match '%s'", dev, nvRegex)
			continue
		}
		devices = append(devices, &pluginapi.Device{ID: devId, Health: pluginapi.Healthy})
	}
	return
}
