package main

import (
	"fmt"
	"net"
	"server/PubSub"
)

func main() {
	server := PubSub.NewServer("clients.json")
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		return
	}
	defer ln.Close()

	fmt.Println("Server started on :8080")

	// Start the console command handler
	go PubSub.HandleServerCommands(server)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			continue
		}
		fmt.Println("New client connected")
		go server.HandleConnection(conn)
	}
}
