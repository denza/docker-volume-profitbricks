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
	cmd := exec.Command("lsblk", "-o", "MOUNTPOINT,NAME", "-J")
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

func (m Utilities) GetDeviceName() (string, error) {
	deviceBaseName := "/dev/%s"

	var stdOut, stdErr bytes.Buffer
	cmd := exec.Command("lsblk", "-o", "MOUNTPOINT,NAME", "-J")
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Error: %s, %s", err.Error(), stdErr.String())
	}

	resultObj := &Result{}

	json.Unmarshal(stdOut.Bytes(), resultObj)

	for _, b := range resultObj.Blockdevices {
		if b.Mountpoint == "" && len(b.Children) == 0 {
			return fmt.Sprintf(deviceBaseName, b.Name), nil
		}
	}
	return "", err
}

type Result struct {
	Blockdevices []struct {
		Mountpoint string `json:"mountpoint"`
		Name       string `json:"name"`
		Children   []struct {
			Mountpoint string `json:"mountpoint"`
			Name       string `json:"name"`
		} `json:"children,omitempty"`
	} `json:"blockdevices"`
}
