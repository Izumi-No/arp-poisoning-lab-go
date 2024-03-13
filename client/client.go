package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

type sharedKeys struct {
	keys map[uuid.UUID][]byte
}

func NewSharedKeys() *sharedKeys {
	return &sharedKeys{
		keys: make(map[uuid.UUID][]byte),
	}
}

func (s *sharedKeys) AddKey(name uuid.UUID, key []byte) {
	s.keys[name] = key
}

func (s *sharedKeys) GetKey(name uuid.UUID) []byte {
	return s.keys[name]
}

func (s *sharedKeys) Has(key uuid.UUID) bool {
	_, ok := s.keys[key]
	return ok
}
func main() {
	// Initialize shared keys map
	sharedKeys := NewSharedKeys()

	// Generate private key for this client
	privateKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		fmt.Println("Error generating private key:", err)
		return
	}

	// Get the public key corresponding to the private key
	publicKey := privateKey.PublicKey()

	// Connect to the server
	conn, err := net.Dial("tcp", "localhost:3000")
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	var id uuid.UUID

	// Start handling terminal input in a separate goroutine
	go handleTerminal(conn, &id, sharedKeys)

	for {
		message := make([]byte, 4096)
		n, err := conn.Read(message)
		if err != nil {
			fmt.Println("Error reading:", err)
			continue // Continue waiting for messages even after an error
		}

		// Decode the received message
		var receivedMessage Event
		err = json.Unmarshal(message[:n], &receivedMessage)
		if err != nil {
			fmt.Println("Error decoding:", err)
			continue // Continue waiting for messages even after an error
		}

		// Process the received message
		if receivedMessage.Event == "setup" {
			// Parse the client ID received during setup
			id, err = uuid.Parse(receivedMessage.Data[0].(string))
			if err != nil {
				fmt.Println("Error parsing UUID:", err)
				continue // Continue waiting for messages even after an error
			}
		}
		if receivedMessage.Event == "KE-OK" {
			// Parse the client ID received during setup
			id, err = uuid.Parse(receivedMessage.Data[0].(string))
			if err != nil {
				fmt.Println("Error parsing UUID:", err)
				continue // Continue waiting for messages even after an error
			}
		}

		if receivedMessage.Event == "KE" {
			fmt.Println("Received KE message from server")

			otherId, err := uuid.Parse(receivedMessage.Data[0].(string))
			if err != nil {
				fmt.Println("Error parsing UUID:", err)
				continue
			}

			if sharedKeys.Has(otherId) {
				continue
			}

			if receivedMessage.Data[0].(string) == id.String() {
				continue
			}

			// Receive public key from the server
			decodedPublicKey, err := base64.StdEncoding.DecodeString(receivedMessage.Data[2].(string))
			if err != nil {
				fmt.Println("Error decoding public key:", err)
				continue
			}
			serverPubKey, err := ecdh.P256().NewPublicKey(decodedPublicKey)
			if err != nil {
				fmt.Println("Error unmarshalling public key:", err)
				continue
			}

			// Perform key exchange
			sharedBytes, err := privateKey.ECDH(serverPubKey)
			if err != nil {
				fmt.Println("Error generating shared key:", err)
				continue
			}

			// Store shared key in the map
			sharedKeys.AddKey(otherId, sharedBytes)

			// Send KE-OK message to server
			eventACK := Event{
				Event: "KE-OK",
				Data:  []interface{}{id.String(), receivedMessage.Data[0]},
			}

			data, err := json.Marshal(eventACK)
			if err != nil {
				fmt.Println("Error encoding KE-OK message:", err)
				return
			}

			_, err = conn.Write(data)
			if err != nil {
				fmt.Println("Error sending KE-OK message:", err)
				return
			}

			fmt.Println("Sent KE-OK message to server")

			event := Event{
				Event: "KE",
				Data:  []interface{}{id.String(), receivedMessage.Data[0], base64.StdEncoding.EncodeToString(publicKey.Bytes())},
			}

			data, err = json.Marshal(event)
			if err != nil {
				fmt.Println("Error encoding KE message:", err)
				return
			}

			_, err = conn.Write(data)
			if err != nil {
				fmt.Println("Error sending KE message:", err)
				return
			}

		}

		if receivedMessage.Event == "message" {
			fmt.Println(receivedMessage.Data[0], " sent:", receivedMessage.Data[2])
		}
		if receivedMessage.Event == "message-encrypted" {

			encriptedMessage := receivedMessage.Data[2].(string)
			senderId, err := uuid.Parse(receivedMessage.Data[0].(string))
			if err != nil {
				fmt.Println("Error parsing UUID:", err)
				continue
			}

			decodedMessage, err := base64.StdEncoding.DecodeString(encriptedMessage)

			if err != nil {
				fmt.Println("Error decoding message:", err)
				continue
			}

			// get shared key
			sharedKey := sharedKeys.GetKey(senderId)

			block, err := aes.NewCipher(sharedKey)
			if err != nil {
				fmt.Println("Error creating new cipher:", err)
				continue
			}

			if len(decodedMessage) < aes.BlockSize {
				fmt.Println("Error decoding message:", err)
				continue
			}

			iv := decodedMessage[:aes.BlockSize]
			decodedMessage = decodedMessage[aes.BlockSize:]

			mode := cipher.NewCBCDecrypter(block, iv)

			mode.CryptBlocks(decodedMessage, decodedMessage)

			decodedMessage = unpadPKCS7(decodedMessage)

			fmt.Println(senderId, " sent:", string(decodedMessage))

		}

		if receivedMessage.Event == "discover" {
			// List all client UUIDs
			fmt.Println("Clients:")
			for _, client := range receivedMessage.Data[0].([]interface{}) {
				clientID, err := uuid.Parse(client.(string))
				if err != nil {
					fmt.Println("Error parsing UUID:", err)
					continue
				}

				// string from byte publicKey.Bytes()
				encodedPublicKey := base64.StdEncoding.EncodeToString(publicKey.Bytes())

				// Send KE message to each client
				event := Event{
					Event: "KE",
					Data:  []interface{}{id.String(), clientID.String(), encodedPublicKey},
				}

				data, err := json.Marshal(event)
				if err != nil {
					fmt.Println("Error encoding KE message:", err)
					return
				}

				if clientID != id {
					fmt.Println("Sending KE message to:", clientID)
					_, err = conn.Write(data)
					if err != nil {
						fmt.Println("Error sending KE message:", err)
						return
					}
				}
			}
		}
	}
}

func handleTerminal(conn net.Conn, id *uuid.UUID, sharedKeys *sharedKeys) {
	for {
		var command string
		fmt.Print("> ")
		fmt.Scanln(&command)

		switch command {
		case "exit":
			conn.Close()
			return

		case "broadcast":
			var message string
			fmt.Print("Message: ")
			fmt.Scanln(&message)

			event := Event{
				Event: "broadcast",
				Data:  []interface{}{id.String(), message},
			}

			data, err := json.Marshal(event)
			if err != nil {
				fmt.Println("Error encoding:", err)
				continue // Continue waiting for input even after an error
			}

			_, err = conn.Write(data)
			if err != nil {
				fmt.Println("Error sending message:", err)
				return // End the goroutine if there's an error sending the message
			}

		case "discover":
			event := Event{
				Event: "discover",
				Data: []interface{}{
					id.String(),
				},
			}

			data, err := json.Marshal(event)
			if err != nil {
				fmt.Println("Error encoding:", err)
				continue // Continue waiting for input even after an error
			}

			_, err = conn.Write(data)
			if err != nil {
				fmt.Println("Error sending message:", err)
				return // End the goroutine if there's an error sending the message
			}

		case "private":
			var message string
			var targetID string
			fmt.Print("Target ID: ")
			fmt.Scanln(&targetID)
			fmt.Print("Message: ")
			fmt.Scanln(&message)

			event := Event{
				Event: "message",
				Data:  []interface{}{id.String(), targetID, message},
			}

			data, err := json.Marshal(event)
			if err != nil {
				fmt.Println("Error encoding:", err)
				continue // Continue waiting for input even after an error
			}

			_, err = conn.Write(data)
			if err != nil {
				fmt.Println("Error sending message:", err)
				return // End the goroutine if there's an error sending the message
			}
		case "private-crypto":
			var message string
			var targetID string
			fmt.Print("Target ID: ")
			fmt.Scanln(&targetID)
			fmt.Print("Message: ")
			fmt.Scanln(&message)

			targetUUID, err := uuid.Parse(targetID)
			if err != nil {
				fmt.Println("Error parsing UUID:", err)
				continue
			}

			// Get shared key
			sharedKey := sharedKeys.GetKey(targetUUID)
			if sharedKey == nil {
				fmt.Println("No shared key found for target ID:", targetID)
				continue
			}

			// Encrypt message
			encryptedMessage, err := encryptMessage([]byte(message), sharedKey)
			if err != nil {
				fmt.Println("Error encrypting message:", err)
				continue
			}

			// Send encrypted message to server
			event := Event{
				Event: "message-encrypted",
				Data:  []interface{}{id.String(), targetID, encryptedMessage},
			}

			sendData(conn, event)

		case "help":
			fmt.Println("Commands:")
			fmt.Println("exit - Close the connection")
			fmt.Println("broadcast <message> - Send a message to all clients")
			fmt.Println("discover - Discover all connected clients")

		default:
			fmt.Println("Unknown command:", command)
			fmt.Println("Type 'help' for a list of commands.")
		}
	}
}

func padPKCS7(data []byte, tamanhoBloco int) []byte {
	preenchimento := tamanhoBloco - len(data)%tamanhoBloco
	padtext := bytes.Repeat([]byte{byte(preenchimento)}, preenchimento)
	return append(data, padtext...)
}

func unpadPKCS7(data []byte) []byte {
	tamanho := len(data)
	ultimoByte := int(data[tamanho-1])
	return data[:(tamanho - ultimoByte)]
}

func encryptMessage(message []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	message = padPKCS7(message, aes.BlockSize)

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	encrypted := make([]byte, aes.BlockSize+len(message))
	copy(encrypted[:aes.BlockSize], iv)

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encrypted[aes.BlockSize:], message)

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func decryptMessage(encrypted string, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	if len(decoded) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := decoded[:aes.BlockSize]
	decoded = decoded[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(decoded, decoded)

	return unpadPKCS7(decoded), nil
}

func sendData(conn net.Conn, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		fmt.Println("Error encoding event:", err)
		return
	}

	_, err = conn.Write(data)
	if err != nil {
		fmt.Println("Error sending event:", err)
		return
	}
}
