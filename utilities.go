package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"os/exec"
	"strings"
)

type Utilities struct {
}

func NewUtilities() *Utilities {
	return &Utilities{}
}

func (m Utilities) MountVolume(volumeName string, mountPoint string) error {
	log.Infof("Mounting volume %s at %s", volumeName, mountPoint)

	var stdOut, stdErr bytes.Buffer
	cmd := exec.Command("mount", volumeName, mountPoint)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	log.Infof("Mount stdout: %s", stdOut.String())

	if err != nil {
		return fmt.Errorf("Error occured while mounting %s: %s", volumeName, stdErr.String())
	}
	return err
}

func (m Utilities) UnmountVolume(mountPoint string) error {
	log.Infof("Unmounting volume %s ", mountPoint)
	var stdOut, stdErr bytes.Buffer
	cmd := exec.Command("umount", mountPoint)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	err := cmd.Run()
	log.Infof("Umount stdout: %s", stdOut.String())

	if err != nil {
		return fmt.Errorf("Error occured while unmounting %s: %s", mountPoint, stdErr.String())
	}
	return err
}

func (m Utilities) FormatVolume(volumeName string) error {
	cmd := exec.Command("mkfs.ext4", volumeName)
	return cmd.Run()
}

func (m Utilities) GetServerId() (string, error) {
	output, err := ioutil.ReadFile("/sys/devices/virtual/dmi/id/product_uuid")
	toReturn := string(output)
	return strings.TrimSpace(toReturn), err
}

func (m Utilities) WriteLsblk(metadataPath string, result Result) error {
	jsn, err := json.Marshal(result)
	if err != nil {
		return err
	}
	ioutil.WriteFile(metadataPath+"metadata.pb", jsn, 0644)

	return err
}

func (m Utilities) getNewLsblk() (Result, error) {
	cmd := exec.Command("lsblk", "-P", "-o", "NAME,MOUNTPOINT,TYPE")

	data, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("Error: %s", err.Error())
	}
	result := []*Device{}
	devices := strings.Split(string(data), "\n")
	fmt.Println("DATA", string(data))
	for _, device := range devices {
		parsed := parseDevice(device)
		if parsed != nil {
			result = append(result, parsed)
		}
	}

	return Result{Devices: result}, err
}

func parseDevice(device string) *Device {
	raw := strings.Split(device, " ")
	if len(raw) == 3 {
		name := strings.Split(raw[0], "=")[1]
		mountpoint := strings.Split(raw[1], "=")[1]
		_type := strings.Split(raw[2], "=")[1]

		d := &Device{
			Name:       strings.Trim(name, `"`),
			Mountpoint: strings.Trim(mountpoint, `"`),
			Type:       strings.Trim(_type, `"`),
		}
		return d
	}
	return nil
}
func (m Utilities) getOldLsblk(metadataPath string) (Result, error) {
	data, err := ioutil.ReadFile(metadataPath + "metadata.pb")
	if err != nil {
		return Result{}, err
	}

	toReturn := Result{}
	err = json.Unmarshal(data, &toReturn)
	if err != nil {
		return Result{}, err
	}

	return toReturn, err
}

func (m Utilities) GetDeviceName(metadataPath string) (string, error) {
	deviceBaseName := "/dev/%s"

	old_list, _ := m.getOldLsblk(metadataPath)
	//if err != nil {
	//	return "", err
	//}

	new_list, err := m.getNewLsblk()

	diff := difference(old_list, new_list)

	if len(diff.Devices) > 1 {
		return "", fmt.Errorf("There is more than %s new devices.", len(diff.Devices))
	}

	return fmt.Sprintf(deviceBaseName, diff.Devices[0].Name), err
}

type Result struct {
	Devices []*Device `json:"blockdevices"`
}

type Device struct {
	Name       string
	Type       string
	Mountpoint string
}

func difference(oldV, newV Result) (toreturn Result) {
	var (
		lenMin  int
		longest Result
	)
	// Determine the shortest length and the longest slice
	if len(oldV.Devices) == 0 {
		toreturn.Devices = append(toreturn.Devices, newV.Devices[len(newV.Devices)-1])
	} else if len(oldV.Devices) < len(newV.Devices) {
		lenMin = len(oldV.Devices)
		longest = newV

	} else {
		lenMin = len(newV.Devices)
		longest = oldV

	}

	// compare common indeces
	for i := 0; i < lenMin; i++ {
		if newV.Devices[i] == nil {
			continue
		}

		if oldV.Devices[i].Name != newV.Devices[i].Name {
			toreturn.Devices = append(toreturn.Devices, newV.Devices[i])
		}

	}

	// add indeces not in common
	for _, v := range longest.Devices[lenMin:] {
		toreturn.Devices = append(toreturn.Devices, v)

	}
	return toreturn
}
