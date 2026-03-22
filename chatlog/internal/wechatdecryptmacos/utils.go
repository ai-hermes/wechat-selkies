package wechatdecryptmacos

import (
	"bytes"
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

func FindWeChatPID() (int, error) {
	cmd := exec.Command("pgrep", "-x", "WeChat")
	out, err := cmd.Output()
	if err != nil {
		return 0, errors.New("WeChat not running or pgrep failed")
	}
	fields := bytes.Fields(out)
	if len(fields) == 0 {
		return 0, errors.New("WeChat not running")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(fields[0])))
	if err != nil {
		return 0, err
	}
	return pid, nil
}
