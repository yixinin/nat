package tunnel

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	hostsSuffix = "# NNAT"
)

func setHosts(fqdn string) bool {
	var h = fmt.Sprintf("127.0.0.1 %s #", fqdn)
	data, err := os.ReadFile(HostsFile)
	if err != nil {
		logrus.Errorf("read hosts file:%s error:%v", HostsFile, err)
		return false
	}
	lines := strings.Split(string(data), LineSep)
	for i, line := range lines {
		if strings.Contains(line, h) {
			lines[i] = line
			break
		}
	}

	lines = append(lines, h)
	err = os.WriteFile(HostsFile, []byte(strings.Join(lines, LineSep)), os.ModePerm)
	if err != nil {
		logrus.Errorf("write hosts file:%s error:%v", HostsFile, err)
		return false
	}
	return true
}

func cleanHosts(fqdn string) {
	var h = fmt.Sprintf("127.0.0.1 %s #", fqdn)
	data, err := os.ReadFile(HostsFile)
	if err != nil {
		logrus.Errorf("read hosts file:%s error:%v", HostsFile, err)
		return
	}
	lines := strings.Split(string(data), LineSep)
	var idx = -1
	for i, line := range lines {
		if strings.Contains(line, h) {
			idx = i
			break
		}
	}
	if idx != -1 {
		lines = append(lines[:idx], lines[idx+1:]...)
	}

	err = os.WriteFile(HostsFile, []byte(strings.Join(lines, LineSep)), os.ModePerm)
	if err != nil {
		logrus.Errorf("clean hosts file:%s error:%v", HostsFile, err)
		return
	}
}
