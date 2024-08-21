package main

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/google/uuid"
)

type Event struct {
	Event string        `json:"event"`
	Data  []interface{} `json:"data"`
}

func (e *Event) String() string {
	data, _ := json.Marshal(e)
	return string(data)
}

type Sockets struct {
	Sockets map[uuid.UUID]net.Conn
}

func NewSockets() *Sockets {
	return &Sockets{
		Sockets: make(map[uuid.UUID]net.Conn),
	}
}

func (s *Sockets) AddSocket(name uuid.UUID, conn net.Conn) {
	s.Sockets[name] = conn
}

func (s *Sockets) RemoveSocket(name uuid.UUID) {
	delete(s.Sockets, name)
}

func (s *Sockets) GetSocket(name uuid.UUID) net.Conn {
	return s.Sockets[name]
}

func (s *Sockets) GetSockets() map[uuid.UUID]net.Conn {
	return s.Sockets
}
func (s *Sockets) GetSocketIDs() []uuid.UUID {
	keys := make([]uuid.UUID, 0, len(s.Sockets))
	for k := range s.Sockets {
		keys = append(keys, k)
	}
	return keys
}

func main() {

	fmt.Println("Server is running on port 3000")
	listener, err := net.Listen("tcp", "0.0.0.0:3000")
	if err != nil {
		fmt.Println("Error listening", err.Error())
		return
	}

	sockets := NewSockets()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting", err.Error())
			return
		}
		id := uuid.New()
		setupMessage := Event{
			Event: "setup",
			Data:  []interface{}{id.String()},
		}

		conn.Write([]byte(setupMessage.String()))

		sockets.AddSocket(id, conn)

		go handleClient(conn, id, sockets)

	}

}

func handleClient(conn net.Conn, id uuid.UUID, sockets *Sockets) {
	defer conn.Close()
	defer func() {
		// Remove a conexão dos sockets quando a função terminar
		sockets.RemoveSocket(id)
		fmt.Println("Connection closed for client:", id)
	}()

	for {
		message := make([]byte, 4096)
		n, err := conn.Read(message)
		if err != nil {
			fmt.Println("Error reading", err.Error())
			return
		}

		//decode message
		recievedMessage := Event{}
		err = json.Unmarshal(message[:n], &recievedMessage)

		if err != nil {
			fmt.Println("Error decoding", err.Error())
			continue // Continue lendo mensagens mesmo se houver um erro de decodificação
		}

		switch recievedMessage.Event {
		case "discover":
			discoverMessage := Event{
				Event: "discover",
				Data:  []interface{}{sockets.GetSocketIDs()},
			}
			conn.Write([]byte(discoverMessage.String()))

		case "broadcast":
			for _, conn := range sockets.GetSockets() {
				if recievedMessage.Data[0].(string) == "00000000-0000-0000-0000-000000000000" {
					continue
				}
				_, err = conn.Write(message[:n])
				if err != nil {
					fmt.Println("Error sending message", err)
					return
				}
			}

		case "message", "message-encrypted", "KE":
			if len(recievedMessage.Data) > 2 {
				recipientID, err := uuid.Parse(recievedMessage.Data[1].(string))
				if err != nil {
					fmt.Println("Error parsing UUID", err)
					continue
				}

				recipientConn := sockets.GetSocket(recipientID)
				if recipientConn == nil {
					fmt.Println("Recipient not found")
					continue
				}

				_, err = recipientConn.Write(message[:n])
				if err != nil {
					fmt.Println("Error sending message", err)
					return
				}
			} else if recievedMessage.Event == "KE-OK" {
				recipientID, err := uuid.Parse(recievedMessage.Data[1].(string))
				if err != nil {
					fmt.Println("Error parsing UUID", err)
					continue
				}

				recipientConn := sockets.GetSocket(recipientID)
				if recipientConn == nil {
					fmt.Println("Recipient not found")
					continue
				}

				_, err = recipientConn.Write(message[:n])

				if err != nil {
					fmt.Println("Error sending message", err)
					return
				}
			} else {
				fmt.Println("Invalid message")
			}
		case "KE-OK":

			// Send KE-OK message to destination
			destinationId, err := uuid.Parse(recievedMessage.Data[0].(string))
			if err != nil {
				fmt.Println("Error parsing UUID:", err)
				continue
			}
			destinationConn := sockets.GetSocket(destinationId)
			if destinationConn == nil {
				fmt.Println("Destination not found")
				continue
			}
			_, err =

				destinationConn.Write(message[:n])
			if err != nil {
				fmt.Println("Error sending KE-OK message:", err)
				return
			}
			fmt.Println("Sent KE-OK message to destination")

		default:
			fmt.Println("Unknown event:", recievedMessage.Event)
		}

	}
}
