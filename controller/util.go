package controller

func validateMachineID(id string) bool {
	if len(id) < 10 {
		return false
	}
	return true
}
