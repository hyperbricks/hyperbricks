package main

// An inaccessible process is not evidence of an exited runtime. Keep its
// deployment record until its state can be established.
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	running, err := processRunning(pid)
	return running || err != nil
}
