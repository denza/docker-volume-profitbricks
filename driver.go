package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/profitbricks/profitbricks-sdk-go"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	MetadataDirMode  = 0700
	MetadataFileMode = 0600
	MountDirMode     = os.ModeDir
)

type Driver struct {
	region       string
	dropletID    int
	metadataPath string
	mountPath    string
	datacenterId string
	serverId     string
	size         int
	diskType     string
	utilities    *Utilities
	sync.RWMutex
	volumes map[string]*VolumeState
}

type VolumeState struct {
	volumeId   string
	mountPoint string
	deviceName string
}

func ProfitBricksDriver(utilities *Utilities, args CommandLineArgs) (*Driver, error) {

	profitbricks.SetAuth(*args.profitbricksUsername, *args.profitbricksPassword)

	err := os.MkdirAll(*args.metadataPath, MetadataDirMode)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(*args.mountPath, MountDirMode)
	if err != nil {
		return nil, err
	}

	serverId, err := utilities.GetServerId()

	if err != nil {
		log.Error(err)
		return nil, err
	}

	log.Info("Server ID:", strings.ToLower(serverId))

	return &Driver{
		datacenterId: *args.datacenterId,
		serverId:     strings.ToLower(serverId),
		size:         *args.size,
		diskType:     *args.diskType,
		volumes:      make(map[string]*VolumeState),
		metadataPath: *args.metadataPath,
		utilities:    utilities,
		mountPath:    *args.mountPath,
	}, nil

}

func (d *Driver) Create(r volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()

	vol := profitbricks.Volume{
		Properties: profitbricks.VolumeProperties{
			Size:        d.size,
			Type:        d.diskType,
			LicenceType: "OTHER",
			Name:        fmt.Sprintf("docker-volume-profitbricks:%s", r.Name),
		},
	}
	createresp := profitbricks.CreateVolume(d.datacenterId, vol)
	log.Info(createresp)
	if createresp.StatusCode > 299 {
		log.Errorf("failed to create a volume '%v'", r.Name)
		return volume.Response{Err: string(vol.Response)}
	}

	volumeId := createresp.Id
	log.Info("Volume provisioned:", vol.Properties.Name)

	err := d.waitTillProvisioned(createresp.Headers.Get("Location"))

	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	attachResp := profitbricks.AttachVolume(d.datacenterId, d.serverId, volumeId)
	if attachResp.StatusCode > 299 {
		log.Errorf("Arguments: %s %s %s", d.datacenterId, d.serverId, vol.Id)
		log.Errorf("failed to attach a volume '%v', error msg: %q", r.Name, vol.Response)
		return volume.Response{Err: string(vol.Response)}
	}

	err = d.waitTillProvisioned(attachResp.Headers.Get("Location"))
	log.Info("Volume attached:", attachResp.Properties.Name)

	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	volumeName, err := d.utilities.GetDeviceName()
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	err = d.utilities.FormatVolume(volumeName)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	volumePath := filepath.Join(d.mountPath, volumeName)

	err = os.MkdirAll(volumePath, MountDirMode)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	metadataFilePath := filepath.Join(d.metadataPath, r.Name)

	metadataFile, err := os.Create(metadataFilePath)
	if err != nil {
		log.Errorf("failed to create metadata file '%v' for volume '%v'", metadataFilePath, r.Name)
		return volume.Response{Err: err.Error()}
	}

	err = metadataFile.Chmod(MetadataFileMode)
	if err != nil {
		os.Remove(metadataFilePath)
		log.Errorf("failed to change the mode for the metadata file '%v' for volume '%v'", metadataFilePath, r.Name)
		return volume.Response{Err: err.Error()}
	}

	d.volumes[r.Name] = &VolumeState{
		volumeId:   vol.Id,
		mountPoint: volumePath,
		deviceName: volumeName,
	}

	log.Info("Volumes: ", d.volumes)
	log.Info("Volume: ", d.volumes[r.Name])

	return volume.Response{}
}

func (d *Driver) Mount(r volume.MountRequest) volume.Response {
	d.Lock()
	defer d.Unlock()
	log.Infof("Mounting Volume: %s", r.Name)

	log.Info(d.volumes[r.Name])
	err := d.utilities.MountVolume(d.volumes[r.Name].deviceName, d.volumes[r.Name].mountPoint)
	if err != nil {
		log.Error(err.Error())
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

func (d *Driver) Unmount(r volume.UnmountRequest) volume.Response {
	d.Lock()
	defer d.Unlock()
	log.Info("Unmounting Volume")

	err := d.utilities.UnmountVolume(d.volumes[r.Name].mountPoint)
	if err != nil {
		log.Error("Error occured while unmounting volume", err.Error())
		return volume.Response{Err: err.Error()}
	}
	return volume.Response{}
}

func (d *Driver) List(r volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()
	volumes := []*volume.Volume{}
	log.Info("Getting a Volume")

	for name, state := range d.volumes {
		volumes = append(volumes, &volume.Volume{
			Name:       name,
			Mountpoint: state.mountPoint,
		})
	}
	return volume.Response{Volumes: volumes}
}

func (d *Driver) Get(r volume.Request) volume.Response {
	log.Info("Getting a Volume")

	if d.volumes[r.Name] == nil {
		return volume.Response{}
	}
	vol := &volume.Volume{
		Name:       d.volumes[r.Name].deviceName,
		Mountpoint: d.volumes[r.Name].mountPoint,
	}

	return volume.Response{Volume: vol}
}

func (d *Driver) Remove(r volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()

	vol := &VolumeState{}
	for _, v := range d.volumes {
		if v.deviceName == r.Name {
			vol = v
			break
		}
	}

	log.Infof("Removing volume %s ", r.Name)
	log.Info("Volumes: ", d.volumes)
	log.Info("Volume: ", d.volumes[r.Name])
	log.Infof("Removing volume with parameters: %s, %s, %s", d.datacenterId, d.serverId, vol.volumeId)
	resp := profitbricks.DetachVolume(d.datacenterId, d.serverId, vol.volumeId)
	if resp.StatusCode > 299 {
		log.Errorf("failed to create metadata file '%v' for volume '%v'", d.metadataPath, r.Name)
		return volume.Response{Err: string(resp.Body)}
	}

	err := d.waitTillProvisioned(resp.Headers.Get("Location"))
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	resp = profitbricks.DeleteVolume(d.datacenterId, d.volumes[r.Name].volumeId)
	if resp.StatusCode > 299 {
		log.Errorf("failed to create metadata file '%v' for volume '%v'", d.metadataPath, r.Name)
		return volume.Response{Err: string(resp.Body)}
	}

	err = d.waitTillProvisioned(resp.Headers.Get("Location"))
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{}
}

func (d *Driver) Path(r volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()

	if state, ok := d.volumes[r.Name]; ok {
		return volume.Response{Mountpoint: state.mountPoint}
	}

	return volume.Response{Err: fmt.Sprintf("Volume %q does not exist", r.Name)}
}

func (d *Driver) Capabilities(r volume.Request) volume.Response {
	log.Infof("[Capabilities]: %+v", r)
	return volume.Response{Capabilities: volume.Capability{Scope: "profitbricks/docker-volume-profitbricks"}}
}

func (d *Driver) waitTillProvisioned(path string) error {

	waitCount := 50

	for i := 0; i < waitCount; i++ {
		request := profitbricks.GetRequestStatus(path)
		log.Infof("Request status: %s", request.Metadata.Status)
		log.Infof("Request status path: %s", path)

		if request.Metadata.Status == "DONE" {
			return nil
		}
		if request.Metadata.Status == "FAILED" {

			return fmt.Errorf("Request failed with following error: %s", request.Metadata.Message)
		}
		time.Sleep(10 * time.Second)
		i++
	}
	return fmt.Errorf("Timeout has expired %s", "")
}

//
//func getDeviceName(deviceNumber int64) string {
//	alphabet := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
//
//	name := fmt.Sprintf("vd%s", alphabet[deviceNumber - 1])
//
//	return name
//}
