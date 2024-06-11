package PubSub

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net"
	"os"
	"strings"
	"sync"
)

type Server struct {
	channels map[string]map[net.Conn]bool
	clients  map[string]net.Conn
	mutex    sync.Mutex
	jsonFile string
}

func NewServer(jsonFile string) *Server {
	return &Server{
		channels: make(map[string]map[net.Conn]bool),
		clients:  make(map[string]net.Conn),
		jsonFile: jsonFile,
	}
}

func (s *Server) saveClients() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	file, err := os.Create(s.jsonFile)
	if err != nil {
		return err
	}
	defer file.Close()

	clients := make(map[string]string)
	for id, conn := range s.clients {
		clients[id] = conn.RemoteAddr().String()
	}
	encoder := json.NewEncoder(file)
	return encoder.Encode(clients)
}

func (s *Server) addClients(conn net.Conn) string {
	id := uuid.New().String()
	s.mutex.Lock()
	s.clients[id] = conn
	s.mutex.Unlock()
	s.saveClients()
	return id
}

func (s *Server) showClient() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	fmt.Println("Active Clients:")
	for id, conn := range s.clients {
		fmt.Printf("ID: %s, Address: %s\n", id, conn.RemoteAddr().String())
	}
}

func (s *Server) removeClient(id string) {
	s.mutex.Lock()
	delete(s.clients, id)
	s.mutex.Unlock()
	s.saveClients()
}

func (s *Server) AddSubscriber(channel string, conn net.Conn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, exists := s.channels[channel]; !exists {
		s.channels[channel] = make(map[net.Conn]bool)
	}
	s.channels[channel][conn] = true
}

func (s *Server) RemoveSubscriber(channel string, conn net.Conn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if subscribers, exists := s.channels[channel]; exists {
		delete(subscribers, conn)
		if len(subscribers) == 0 {
			delete(s.channels, channel)
		}
	}
}

func (s *Server) BroadcastMessage(channel, message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if subscribers, exists := s.channels[channel]; exists {
		for conn := range subscribers {
			_, err := conn.Write([]byte(message + "\n"))
			if err != nil {
				fmt.Printf("Error broadcasting to subscriber: %v\n", err)
				conn.Close()
				delete(subscribers, conn)
			}
		}
	}
}

func (s *Server) PublishMessage(channel, message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if subscribers, exists := s.channels[channel]; exists {
		for conn := range subscribers {
			_, err := conn.Write([]byte(message + "\n"))
			if err != nil {
				fmt.Printf("Error publishing to channel: %v\n", err)
			}
		}
	}
}

func (s *Server) DeleteChannel(channel string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.channels, channel)
	fmt.Printf("Channel %s deleted\n", channel)
}

func (s *Server) ShowChannelsInConsole() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	fmt.Println("Active Channels:")
	count := 0
	for channel := range s.channels {
		fmt.Println(channel)
		count++
	}
	if count == 0 {
		fmt.Println("No channels found")
	}

}

func (s *Server) ShowChannels(conn net.Conn) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	counter := 0
	channels := make([]string, 0, len(s.channels))
	for channel := range s.channels {
		channels = append(channels, channel)
		counter++
	}
	if channels == nil || len(channels) == 0 {
		return 0
	}
	_, err := conn.Write([]byte(strings.Join(channels, "\n") + "\n"))
	if err != nil {
		fmt.Printf("Error sending channel list: %v\n", err)
	}
	return counter
}

func (s *Server) HandleConnection(conn net.Conn) {
	defer conn.Close()
	clientID := s.addClients(conn)
	defer s.removeClient(clientID)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 3)
		command := parts[0]

		switch command {
		case "SUBSCRIBE":
			if len(parts) < 2 {
				continue
			}
			channel := parts[1]
			s.AddSubscriber(channel, conn)
		case "PUBLISH":
			if len(parts) < 3 {
				continue
			}
			channel := parts[1]
			message := parts[2]
			s.PublishMessage(channel, message)
		case "UNSUBSCRIBE":
			if len(parts) < 2 {
				continue
			}
			channel := parts[1]
			s.RemoveSubscriber(channel, conn)
			return
		case "SHOWLIST":

			if s.ShowChannels(conn) == 0 {
				_, _ = conn.Write([]byte("NO CHANNELS AVAILABLE\n"))

			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading from connection: %v\n", err)
	}
}

func HandleServerCommands(server *Server) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 3)
		command := parts[0]

		switch command {
		case "SHOWCLIENT":
			server.showClient()
		case "SUBSCRIBE":
			if len(parts) < 2 {
				fmt.Println("Usage: SUBSCRIBE <channel>")
				continue
			}
			channel := parts[1]
			// Initialize the channel with an empty map of subscribers
			server.mutex.Lock()
			if _, exists := server.channels[channel]; !exists {
				server.channels[channel] = make(map[net.Conn]bool)
			}
			server.mutex.Unlock()
			fmt.Printf("Channel %s created\n", channel)
		case "PUBLISH":
			if len(parts) < 3 {
				fmt.Println("Usage: PUBLISH <channel> <message>")
				continue
			}
			channel := parts[1]
			message := parts[2]
			server.PublishMessage(channel, message)
		case "DELETE":
			if len(parts) < 2 {
				fmt.Println("Usage: DELETE <channel>")
				continue
			}
			channel := parts[1]
			server.DeleteChannel(channel)
		case "SHOWCHANNEL":
			server.ShowChannelsInConsole()

		default:
			fmt.Println("Unknown command")
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading from console: %v\n", err)
	}
}
