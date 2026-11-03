package main

import (
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func publish(root, name, value string) error {
	path := filepath.Join(root, name)
	if err := os.WriteFile(path+".next", []byte(value), 0o600); err != nil {
		return err
	}
	return os.Rename(path+".next", path)
}

func main() {
	root := os.Getenv("CODEX_TEST_PROCESS_ROOT")
	if root == "" {
		os.Exit(2)
	}
	if len(os.Args) > 1 && os.Args[1] == "heartbeat-child" {
		for i := range 1000 {
			if _, err := os.Stat(filepath.Join(root, "stop")); err == nil {
				return
			}
			if err := publish(root, "heartbeat", strconv.Itoa(i)); err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		return
	}
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	child := exec.Command(executable, "heartbeat-child")
	if err := child.Start(); err != nil {
		panic(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	if err := publish(root, "pids", strconv.Itoa(os.Getpid())+" "+strconv.Itoa(child.Process.Pid)); err != nil {
		panic(err)
	}
	select {
	case <-signals:
	case <-time.After(10 * time.Second):
	}
}
