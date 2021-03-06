// +build linux

package main

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"log/syslog"
	"os"

	"github.com/godbus/dbus"
)

var conn *dbus.Conn

type State struct {
	Version string `json:"version"`
	ID      string `json:"id"`
	Pid     int    `json:"pid"`
	Root    string `json:"root"`
}

func Validate(id string) (string, error) {
	for len(id) < 32 {
		id += "0"
	}
	return hex.EncodeToString([]byte(id)), nil
}

// RegisterMachine with systemd on the host system
func RegisterMachine(name string, id string, pid int, root_directory string) error {
	var (
		av  []byte
		err error
	)
	if conn == nil {
		conn, err = dbus.SystemBus()
		if err != nil {
			return err
		}
	}

	av, err = hex.DecodeString(id[0:32])
	if err != nil {
		return err
	}
	obj := conn.Object("org.freedesktop.machine1", "/org/freedesktop/machine1")
	service := os.Getenv("container")
	if service == "" {
		service = "runc"
	}
	return obj.Call("org.freedesktop.machine1.Manager.RegisterMachine", 0, name[0:32], av, service, "container", uint32(pid), root_directory).Err
}

// TerminateMachine registered with systemd on the host system
func TerminateMachine(name string) error {
	var err error
	if conn == nil {
		conn, err = dbus.SystemBus()
		if err != nil {
			return err
		}
	}
	obj := conn.Object("org.freedesktop.machine1", "/org/freedesktop/machine1")
	return obj.Call("org.freedesktop.machine1.Manager.TerminateMachine", 0, name).Err
	return nil
}

func main() {
	var state State
	logwriter, err := syslog.New(syslog.LOG_NOTICE, "oci-register-machine")
	if err == nil {
		log.SetOutput(logwriter)
	}
	command := os.Args[1]
	if err := json.NewDecoder(os.Stdin).Decode(&state); err != nil {
		log.Fatalf("RegisterMachine Failed %v", err.Error())
	}

	log.Printf("Register machine: %s %s %d %s", command, state.ID, state.Pid, state.Root)
	// ensure id is a hex string at least 32 chars
	passId, err := Validate(state.ID)
	if err != nil {
		log.Fatalf("RegisterMachine Failed %v", err.Error())
	}

	switch command {
	case "prestart":
		{
			if err = RegisterMachine(state.ID, passId, int(state.Pid), state.Root); err != nil {
				log.Fatalf("Register machine failed: %v", err)
			}
			return
		}
	case "poststop":
		{
			if err := TerminateMachine(state.ID); err != nil {
				log.Fatalf("TerminateMachine failed: %v", err)
			}
			return
		}
		log.Fatalf("Invalid command %q must be prestart|poststop", command)
	}
}
