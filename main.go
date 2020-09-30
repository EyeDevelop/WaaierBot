package main

import (
	whatsapp "github.com/Rhymen/go-whatsapp"
	qrT "github.com/Baozisoftware/qrcode-terminal-go"
	"fmt"
	"time"
	"os"
	"os/signal"
	"syscall"
	"encoding/gob"
	"./handlers"
)

func main() {
	// Create a connection to WhatsApp.
	wac, err := whatsapp.NewConn(20 * time.Second);
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not create a connection to WhatsApp: %v\n", err)
		return
	}

	wac.AddHandler(&handlers.WaaierHandler{Conn: wac, StartTime: uint64(time.Now().Unix())})

	// Login to WhatsApp.
	if err = login(wac); err != nil {
		fmt.Fprintf(os.Stderr, "Could not login to WhatsApp: %v\n", err)
		return
	}

	// Verify phone connectivity.
	pong, err := wac.AdminTest()
	if !pong || err != nil {
		fmt.Fprintf(os.Stderr, "Cannot connect to phone: %v.\n", err)
		return
	}

	// Wait for a SIGTERM.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Disconnect safely.
	fmt.Println("Shutting down...")
	session, err := wac.Disconnect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to disconnect from WhatsApp: %v\n", err)
		return
	}

	if err := writeSession(session); err != nil {
		fmt.Fprintf(os.Stderr, "Could not store session: %v\n", err)
		return
	}
}

func login(conn *whatsapp.Conn) error {
	// Try to load previous session.
	session, err := readSession()
	if err != nil {
		// No previous session, or error loading. Make a new one.
		qr := make(chan string)
		go func() {
			term := qrT.New()
			term.Get(<-qr).Print()
		}()

		// Create a new session and store it.
		session, err = conn.Login(qr)
		if err != nil {
			return fmt.Errorf("cannot login: %v", err)
		}
	} else {
		session, err = conn.RestoreWithSession(session)
		if err != nil {
			return fmt.Errorf("cannot restore: %v", err)
		}
	}

	err = writeSession(session)
	return err
}

// Read the session from the session file
// and try to reconnect to WhatsApp.
func readSession() (whatsapp.Session, error) {
	// Create a dummy session object.
	session := whatsapp.Session{}

	// Try to read the previous session file.
	file, err := os.Open(".session.gob")
	defer file.Close()

	if err != nil {
		return session, err
	}

	// Decode and return the session.
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&session)

	return session, err
}

// Store the session to a file for later
// restoring.
func writeSession(session whatsapp.Session) error {
	// Open the file and write.
	file, err := os.OpenFile(".session.gob", os.O_CREATE|os.O_RDWR, 0600)
	defer file.Close()

	if err != nil {
		return err
	}

	// Encode the session and store it in the file.
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(session)
	
	return err
}