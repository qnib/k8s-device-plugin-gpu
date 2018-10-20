package qniblib // import "github.com/qnib/k8s-device-plugin-gpu/libs"

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"time"
	"github.com/zpatrick/go-config"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	resourceName           = "qnib.org/gpu"
	serverSock             = pluginapi.DevicePluginPath + "qnib-gpu.sock"
	envDisableHealthChecks = "DP_DISABLE_HEALTHCHECKS"
)

// QnibGPUDevicePlugin implements the Kubernetes device plugin API
type QnibGPUDevicePlugin struct {
	devs   []*pluginapi.Device
	socket string
	cfg		*config.Config
	stop   chan interface{}
	health chan *pluginapi.Device
	server *grpc.Server
}



// NewQnibGPUDevicePlugin returns an initialized QnibGPUDevicePlugin
func NewQnibGPUDevicePlugin(cfg string) *QnibGPUDevicePlugin {
	c, _ := NewConfig(cfg)
	return &QnibGPUDevicePlugin{
		devs: 	GetDevices(c),
		socket: serverSock,
		cfg: 	c,
		stop:   make(chan interface{}),
		health: make(chan *pluginapi.Device),
	}
}

func (m *QnibGPUDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

// dial establishes the gRPC communication with the registered device plugin.
func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

// Start starts the gRPC server of the device plugin
func (m *QnibGPUDevicePlugin) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		return err
	}

	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(m.server, m)

	go m.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	conn, err := dial(m.socket, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	go m.healthcheck()

	return nil
}

// Stop stops the gRPC server
func (m *QnibGPUDevicePlugin) Stop() error {
	if m.server == nil {
		return nil
	}

	m.server.Stop()
	m.server = nil
	close(m.stop)

	return m.cleanup()
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (m *QnibGPUDevicePlugin) Register(kubeletEndpoint, resourceName string) error {
	conn, err := dial(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(m.socket),
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

// ListAndWatch lists devices and update that list according to the health status
func (m *QnibGPUDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})

	for {
		select {
		case <-m.stop:
			return nil
		case d := <-m.health:
			// FIXME: there is no way to recover from the Unhealthy state.
			d.Health = pluginapi.Unhealthy
			s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
		}
	}
}

func (m *QnibGPUDevicePlugin) unhealthy(dev *pluginapi.Device) {
	m.health <- dev
}

// Allocate which return list of devices.
func (m *QnibGPUDevicePlugin) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		devs := []*pluginapi.DeviceSpec{}
		envs := map[string]string{}
		mnts := []*pluginapi.Mount{}
		// Devs
		sidekickDevs, _ := m.cfg.StringOr("devices.sidekicks", "")
		for _, sidekick := range strings.Split(sidekickDevs, ",") {
			devs = append(devs, &pluginapi.DeviceSpec{sidekick, sidekick, "rwm"})
		}
		for _, devId := range req.DevicesIDs {
			dpath := fmt.Sprintf("/dev/nvidia%s", devId)
			devs = append(devs, &pluginapi.DeviceSpec{dpath, dpath, "rwm"})
		}
		// Environment
		envLibs, _ := m.cfg.StringOr("environment.libs", "")
		for _, envLib := range strings.Split(envLibs, ",") {
			s := strings.Split(envLib, "=")
			if len(s) != 2 {
				continue
			}
			envs[s[0]] = s[1]
		}
		// Mounts
		libMnts, _ := m.cfg.StringOr("mounts.libs", "")
		for _, libMnt := range strings.Split(libMnts, ",") {
			s := strings.Split(libMnt, ":")
			switch len(s) {
			case 2:
				mnts = append(mnts, &pluginapi.Mount{s[1], s[0], true})
			default:
				mnts = append(mnts, &pluginapi.Mount{libMnt, libMnt, true})
			}

		}
		binMnts, _ := m.cfg.StringOr("mounts.bins", "")
		for _, binMnt := range strings.Split(binMnts, ",") {
			mnts = append(mnts, &pluginapi.Mount{binMnt, binMnt, true})
		}
		response := pluginapi.ContainerAllocateResponse{
			Envs: envs,
			Devices: devs,
			Mounts: mnts,
		}
		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}

	return &responses, nil
}

func (m *QnibGPUDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (m *QnibGPUDevicePlugin) cleanup() error {
	if err := os.Remove(m.socket); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (m *QnibGPUDevicePlugin) healthcheck() {
	// TODO: skip this
}

// Serve starts the gRPC server and register the device plugin to Kubelet
func (m *QnibGPUDevicePlugin) Serve() error {
	err := m.Start()
	if err != nil {
		log.Printf("Could not start device plugin: %s", err)
		return err
	}
	log.Println("Starting to serve on", m.socket)

	err = m.Register(pluginapi.KubeletSocket, resourceName)
	if err != nil {
		log.Printf("Could not register device plugin: %s", err)
		m.Stop()
		return err
	}
	log.Println("Registered device plugin with Kubelet")

	return nil
}
