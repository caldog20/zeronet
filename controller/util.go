package controller

import "regexp"

const MachineIDLen = 64

var alphanumeric = regexp.MustCompile("^[a-zA-Z0-9_]*$")

func validateMachineID(id string) bool {
	return true
	if !alphanumeric.MatchString(id) {
		return false
	}
	if len(id) < MachineIDLen {
		return false
	}

	return true
}
