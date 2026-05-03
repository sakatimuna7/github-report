package utils

import (
	"os/exec"
	"strings"
)

func Sh(c string, a ...string) string {
	o, _ := exec.Command(c, a...).Output()
	return strings.TrimSpace(string(o))
}
