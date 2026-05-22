package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const (
	pidFile = "/tmp/perrus-cli.pid"
	logFile = "/tmp/perrus-cli.log"
)

func startDaemon(port int, configPath string) {
	if isDaemonRunning() {
		fmt.Println("daemon is already running")
		return
	}

	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot resolve executable: %v\n", err)
		os.Exit(1)
	}

	lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open log file: %v\n", err)
		os.Exit(1)
	}
	defer lf.Close()

	cmd := exec.Command(execPath, "start",
		fmt.Sprintf("-port=%d", port),
		fmt.Sprintf("-config=%s", configPath),
	)
	cmd.Stdout = lf
	cmd.Stderr = lf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "cannot write pid file: %v\n", err)
		cmd.Process.Kill()
		os.Exit(1)
	}

	fmt.Printf("daemon started (pid %d)\n", cmd.Process.Pid)
	fmt.Printf("dashboard: http://localhost:%d\n", port)
	fmt.Printf("logs:      %s\n", logFile)
}

func stopDaemon() {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("daemon is not running")
			return
		}
		fmt.Fprintf(os.Stderr, "cannot read pid file: %v\n", err)
		os.Exit(1)
	}

	var pid int
	fmt.Sscanf(string(data), "%d", &pid)

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot find process %d: %v\n", pid, err)
		os.Exit(1)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "cannot stop process: %v\n", err)
		os.Exit(1)
	}

	os.Remove(pidFile)
	fmt.Printf("daemon stopped (pid %d)\n", pid)
}

func checkDaemonStatus() {
	if isDaemonRunning() {
		data, _ := os.ReadFile(pidFile)
		fmt.Printf("daemon running (pid %s)\n", string(data))
		fmt.Printf("logs: %s\n", logFile)
	} else {
		fmt.Println("daemon is not running")
	}
}

func isDaemonRunning() bool {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}
	var pid int
	fmt.Sscanf(string(data), "%d", &pid)
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		os.Remove(pidFile)
		return false
	}
	return true
}
