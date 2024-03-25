package main

import (
	"net"
	"testing"
	"time"
)

func TestMaxConnections(t *testing.T) {
	s := NewServer("localhost:4000")
	go s.Start()
	time.Sleep(1 * time.Second)
	d := net.Dialer{Timeout: 100 * time.Millisecond, Deadline: time.Now().Add(5 * time.Second)}
	for i := 0; i < maxConnections; i++ {
		// fmt.Println("Connection", i+1)

		conn, err := d.Dial("tcp", "localhost:4000")
		if err != nil {
			t.Fatalf("Failed to connect to server: %s", err)
		}
		defer conn.Close()
		time.Sleep(100 * time.Millisecond)
	}
	// Attempt one more connection beyond the limit
	// fmt.Println("Last connection")
	_, err := d.Dial("tcp", "localhost:4000")
	if err != nil {
		t.Fatalf("Failed to connect to server: %s", err)
	}
	time.Sleep(2 * time.Second)

	connMutex.Lock()

	if s.connCount > maxConnections {
		// fmt.Println("len(connections) > maxConnections")
		t.Errorf("len(connections) = %d; want %d", s.connCount, maxConnections)
	}
	connMutex.Unlock()
	s.quitch <- true // send a signal to stop the server
}
