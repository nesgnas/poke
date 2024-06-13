package PubSub

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Server struct {
	channels        map[string]map[net.Conn]bool
	clients         map[string]net.Conn
	mutex           sync.Mutex
	jsonFile        string
	broadcastTicker *time.Ticker
}

type Pokemon struct {
	UID string `json:"uid"`
	ID  int    `json:"id"`
	Exp int    `json:"exp"`
	EV  float64    `json:"ev"`
	LV  int    `json:"lv"`
}

func createRandomPokemon() Pokemon {
	return Pokemon{
		UID: uuid.New().String(),
		ID:  rand.Intn(898) + 1, // Random ID between 0 and 99
		Exp: 0,
		EV:  0.5 + rand.Float64()*0.5,
		LV:  rand.Intn(5) + 1,
	}
}

func createRandomListPokemon(n int) []Pokemon {
	listPokemon := make([]Pokemon, n)
	for i := 0; i < n; i++ {
		listPokemon[i] = createRandomPokemon()
	}
	return listPokemon
}

func NewServer(jsonFile string) *Server {
	server := &Server{
		channels:        make(map[string]map[net.Conn]bool),
		clients:         make(map[string]net.Conn),
		jsonFile:        jsonFile,
		broadcastTicker: time.NewTicker(10 * time.Second),
	}

	//go server.startBroadcasting()

	return server
}

func (s *Server) startBroadcasting() {
	for range s.broadcastTicker.C {
		s.BroadcastToAllClients("This is a periodic message sent every 10 seconds.")
		s.BroadcastToAllClients("REPEAT GET clients.json")
	}
}

func (s *Server) BroadcastToAllClients(message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for clientID, conn := range s.clients {
		_, err := conn.Write([]byte(message + "\n"))
		if err != nil {
			fmt.Printf("Error sending periodic message to client %s: %v\n", clientID, err)
			conn.Close()
			delete(s.clients, clientID)
		}
	}
}

func (s *Server) saveToJSONFile(filename string, data interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(data)
}

func (s *Server) removeClient(id string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Remove the client from the clients map
	delete(s.clients, id)
	fmt.Println("Removed client: " + id)

	// Open the JSON file
	file, err := os.Open(s.jsonFile)
	if err != nil {
		fmt.Printf("Error opening JSON file: %v\n", err)
		return
	}
	defer file.Close()

	// Decode the JSON data
	var jsonData map[string][]map[string]interface{}
	if err := json.NewDecoder(file).Decode(&jsonData); err != nil {
		fmt.Printf("Error decoding JSON data: %v\n", err)
		return
	}

	// Filter out collections with matching uID
	var filteredUsers []map[string]interface{}
	for _, user := range jsonData["user"] {
		if user["uID"] != id {
			filteredUsers = append(filteredUsers, user)
		}
	}

	// Prepare the filtered JSON data
	filteredData := map[string][]map[string]interface{}{
		"user": filteredUsers,
	}

	// Encode and save the filtered JSON data to the file
	file, err = os.Create(s.jsonFile)
	if err != nil {
		fmt.Printf("Error creating JSON file: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(filteredData); err != nil {
		fmt.Printf("Error encoding JSON data: %v\n", err)
		return
	}
	fmt.Println("Client data removed from JSON file.")
}

func (s *Server) saveClients(list []Pokemon) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Open the clients.json file
	file, err := os.Open(s.jsonFile)
	if err != nil {
		return fmt.Errorf("error opening clients.json file: %v", err)
	}
	defer file.Close()

	// Decode the existing users into map A
	var A map[string]interface{}
	if err := json.NewDecoder(file).Decode(&A); err != nil && err != io.EOF {
		return fmt.Errorf("error decoding clients.json file: %v", err)
	}

	// If map A is nil or user key is nil, initialize it as an empty slice
	if A == nil {
		A = make(map[string]interface{})
	}
	if A["user"] == nil {
		A["user"] = []interface{}{}
	}

	// Prepare the user array
	users := make([]map[string]interface{}, 0, len(s.clients))
	for clientID, conn := range s.clients {
		// Check if the client's uID exists in map A
		userData := make(map[string]interface{})
		for _, u := range A["user"].([]interface{}) {
			user := u.(map[string]interface{})
			if user["uID"] == clientID {
				userData = user
				break
			}
		}

		if len(userData) > 0 {
			// Use existing data for this user
			users = append(users, userData)
		} else {
			// Generate new random values for positionX and positionY
			positionX := -1
			positionY := -1
			user := map[string]interface{}{
				"uID":         clientID,
				"connAdd":     conn.RemoteAddr().String(),
				"positionX":   positionX,
				"positionY":   positionY,
				"listPokemon": []interface{}{list},
				"maxValue":    "",
				"spaceLeft":   "",
			}
			users = append(users, user)
		}
	}

	// Update map A with the new users' data
	A["user"] = users

	// Encode and save map A to JSON file
	file, err = os.Create(s.jsonFile)
	if err != nil {
		return fmt.Errorf("error creating clients.json file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(A); err != nil {
		return fmt.Errorf("error encoding clients data to JSON file: %v", err)
	}

	return nil
}

func (s *Server) addClient(conn net.Conn, list []Pokemon) string {
	id := uuid.New().String()
	s.mutex.Lock()
	s.clients[id] = conn
	s.mutex.Unlock()

	// Save the client's data including positionX and positionY
	err := s.saveClients(list)
	if err != nil {
		fmt.Printf("Error saving client data: %v\n", err)
	}

	//s.broadcastClientsJSON(s.jsonFile)
	return id
}

func (s *Server) showClients() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	fmt.Println("Active Clients:")
	for id, conn := range s.clients {
		fmt.Printf("ID: %s, Address: %s\n", id, conn.RemoteAddr().String())
	}
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

} // for Server testing console

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
} // for Client request print into console

func (s *Server) broadcastClientsJSON(fileName string) {

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Open the clients.json file
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("Error opening clients.json file: %v\n", err)
		return
	}
	defer file.Close()

	// Read the file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		fmt.Printf("Error reading"+s.jsonFile+" file: %v\n", err)
		return
	}

	// Broadcast the file content to all connected clients
	for _, conn := range s.clients {
		_, err := conn.Write(fileContent)
		if err != nil {
			fmt.Printf("Error sending clients.json contents to client: %v\n", err)
			// Optionally, handle disconnections if necessary
			conn.Close()
			delete(s.clients, conn.RemoteAddr().String())
		}
	}
} // sent file to Client

func (s *Server) HandleConnection(conn net.Conn) {

	defer conn.Close()

	// Random Pokemon List
	list := createRandomListPokemon(3)

	clientID := s.addClient(conn, list)
	s.BroadcastToAllClients("REPEAT GET clients.json")
	defer func() {
		s.removeClient(clientID)
		s.BroadcastToAllClients("REPEAT GET clients.json")
	}()

	//s.broadcastClientsJSON(s.jsonFile)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 10)
		fmt.Println(parts)
		command := parts[0]
		fmt.Println(command)

		switch command {
		case "SUBSCRIBE":
			if len(parts) < 2 {
				continue
			}
			channel := parts[1]
			s.AddSubscriber(channel, conn)
		case "PUBLISH":
			if len(parts) < 2 {
				continue
			}
			channel := parts[1]

			message := ""
			for i := 0; i < len(parts); i++ {
				message += parts[i] + " "
			}
			fmt.Println(message)
			s.PublishMessage(channel, message)
		case "UNSUBSCRIBE":
			if len(parts) < 2 {
				continue
			}
			channel := parts[1]
			s.RemoveSubscriber(channel, conn)
			return // Exit the loop and close connection
		case "SHOWLIST":

			if s.ShowChannels(conn) == 0 {
				_, _ = conn.Write([]byte("NO CHANNELS AVAILABLE\n"))

			}
		case "EXIT":
			fmt.Println("Exiting...")
			fmt.Println(clientID)
			s.removeClient(clientID)
			fmt.Println("Exiting.")
			s.broadcastClientsJSON(s.jsonFile)
			fmt.Println("Exiting..")

			return // Exit the loop and close connection
		case "GET":
			if len(parts) < 2 {
				continue
			}
			fileName := parts[1]
			s.broadcastClientsJSON(fileName)

		}

	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading from connection: %v\n", err)
	}

	// If scanner.Scan() returns false (client disconnected), the loop will exit and cleanup will be done by the deferred function.
} // for Client

func HandleServerCommands(server *Server) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 3)
		command := parts[0]

		switch command {
		case "SHOWCLIENT":
			server.showClients()
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
} // for Server cmd
