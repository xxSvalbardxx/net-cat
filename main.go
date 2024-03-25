package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	maxConnections = 2
)

var (
	connections = make(map[*net.Conn]string)
	// synchronization primitive used to ensure that only one goroutine accesses the map at a time
	connMutex     sync.Mutex
	messages      []Message
	messagesMutex sync.Mutex
)

type Message struct {
	from    string
	payload string
}

type Server struct {
	listenAddr string
	ln         net.Listener
	connCount  int
	quitch     chan bool
}

func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
		connCount:  0,
		quitch:     make(chan bool),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	s.ln = ln

	fmt.Printf("Server started and listening on %s\n", s.listenAddr)
	log.Printf("Server started and listening on %s\n", s.listenAddr)

	go s.acceptLoop()

	// Wait for the quit signal
	<-s.quitch

	// Clear the connections map
	connMutex.Lock()
	for conn := range connections {
		(*conn).Close()
		delete(connections, conn)
	}
	connMutex.Unlock()

	return nil
}

func (s *Server) acceptLoop() {
	for {

		conn, err := s.ln.Accept()
		if err != nil {
			log.Println("accept error:", err)
			continue
		}
		s.connCount++

		// Rate limiting handling
		if s.connCount > maxConnections {
			log.Println("Max connections reached")
			conn.Write([]byte("Max connections reached. Come back later\n"))
			conn.Close()
			s.connCount--
			continue

		}

		log.Println("New connection from:", conn.RemoteAddr())
		go s.HandleConnection(conn)
	}
}

func (s *Server) HandleConnection(conn net.Conn) {
	conn.Write([]byte("Welcome to TCP-Chat!\n" +
		"		         _nnnn_\n" +
		"		        dGGGGMMb\n" +
		"		       @p~qp~~qMb\n" +
		"		       M|@||@) M|\n" +
		"		       @,----.JM|\n" +
		"		      JS^\\__/  qKL\n" +
		"		     dZP        qKRb\n" +
		"		    dZP          qKKb\n" +
		"		   fZP            SMMb\n" +
		"		   HZM            MMMM\n" +
		"		   FqM            MMMM\n" +
		"		  __| \".        |\\dS\"qML\n" +
		"		  |    `.       | `' \\Zq\n" +
		"		 _)      \\.___.,|     .'\n" +
		"		\\____   )MMMMMP|   .' \n" +
		"		     `-'       `--'\n" +
		"[ENTER YOUR NAME]:"))

	name := readName(conn)
	announceJoin(name, conn)

	conn.Write([]byte(fmt.Sprintf("Welcome \033[31;1;4m%s\033[0m!\n", name)))

	connMutex.Lock()          // Lock the map
	connections[&conn] = name // Add the connection to the map
	connMutex.Unlock()        // Unlock the map
	log.Printf("New connection from %s\n", name)

	// display previous messages if any
	displayPreviousMessages(conn)
	conn.Write([]byte(formatMessage(name, "$:")))

	s.readLoop(conn, name)

	// User cleanup
	announceLeave(name, conn)

	delete(connections, &conn)
	s.connCount--
}

func displayPreviousMessages(conn net.Conn) {
	messagesMutex.Lock()
	for _, msg := range messages {
		if _, err := fmt.Fprintf(conn, "%s\n", formatMessage(msg.from, string(msg.payload))); err != nil {
			log.Printf("Error sending message to %s: %s\n", msg.from, err)
		}
	}
	messagesMutex.Unlock()
}

func readName(conn net.Conn) string {
	fmt.Println("Reading name")
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		log.Println("read name error:", err)
		return "Unknown"
	}
	name_formated := strings.TrimSpace(string(buf[:n]))
	if name_formated == "" {
		return "Anonymoustache"
	}
	return name_formated
}

func (s *Server) readLoop(conn net.Conn, name string) {
	defer conn.Close()
	for {
		buf := make([]byte, 2048)

		n, err := conn.Read(buf)
		if err != nil {
			// Only show the error if it isn't a close connection error
			if err != io.EOF {
				log.Println("read loop error:", err)
			} else {
				fmt.Printf("Connection from %s closed\n", name)
			}

			return
		}

		payload := strings.TrimSpace(string(buf[:n]))
		if payload == "" {
			continue
		}

		msg := Message{
			from:    name,
			payload: payload,
		}

		handleMessage(conn, msg)
	}
}

func handleMessage(conn net.Conn, msg Message) {
	// Store
	messagesMutex.Lock()
	messages = append(messages, msg)
	messagesMutex.Unlock()

	broadcastMessage(formatMessage(msg.from, string(msg.payload)), conn)
}

func main() {
	setLogFile("server.log")
	//[USAGE]: ./TCPChat $port
	var port int

	if len(os.Args) > 2 {
		fmt.Println("[USAGE]: ./TCPChat $port")
		os.Exit(1)
	} else if len(os.Args) == 2 {
		p, err := strconv.Atoi(os.Args[1])
		if err != nil {
			fmt.Println("Invalid port number")
			fmt.Println("[USAGE]: ./TCPChat $port")
			os.Exit(1)
		}
		if p < 1024 || p > 65535 {
			fmt.Println("Port number must be between 1024 and 65535")
			fmt.Println("[USAGE]: ./TCPChat $port")
			os.Exit(1)
		}
		port = p
	} else {
		port = 8989
	}
	server := NewServer("localhost:" + strconv.Itoa(port))

	// Set up channel to listen for interrupt signal (Ctrl+C)
	sigs := make(chan os.Signal, 1) // 1 is for the buffer size
	// Notify sigs channel on SIGINT (Ctrl+C) or SIGTERM (termination signal from outside)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Press Ctrl+C to stop the server")
	go func() {
		<-sigs
		server.quitch <- true
		log.Println("Shutting down server...")
	}()

	// go handleMessages(server.msgch)
	err := server.Start()
	if err != nil {
		log.Fatalln("Failed to start server:", err)
	}
}

func setLogFile(filename string) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		log.Fatalln("Failed to open log file:", err)
	}
	log.SetOutput(file)
}

func formatMessage(name, text string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	name = fmt.Sprintf("\033[32m%s\033[0m", name)
	timestamp = fmt.Sprintf("\033[33m%s\033[0m", timestamp)

	return fmt.Sprintf("[%s][%s]\n\033[35m%s\033[0m", timestamp, name, text)
}

func broadcastMessage(message string, senderConn net.Conn) {
	connMutex.Lock()
	defer connMutex.Unlock()
	for conn := range connections {
		name := connections[conn]
		if *conn != senderConn {
			if _, err := fmt.Fprintf(*conn, "\n%s\n", message); err != nil {
				log.Printf("Error sending message to %s: %s\n", name, err)
			}
		}
		fmt.Fprintf(*conn, "%s", formatMessage(name, "$:"))
	}
}

func announceJoin(name string, conn net.Conn) {
	message := formatMessage("\033[31mServer\033[0m", fmt.Sprintf("\033[32m%s\033[0m \033[34mhas joined the chat\033[0m", name))
	broadcastMessage(message, conn)
}

func announceLeave(name string, conn net.Conn) {
	message := formatMessage("\033[31mServer\033[0m", fmt.Sprintf("\033[32m%s\033[0m \033[34mhas left the chat\033[0m", name))
	broadcastMessage(message, conn)
}
