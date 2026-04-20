package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Client struct {
	conn net.Conn
	name string
}

var (
	clients   = make(map[*Client]bool)
	clientsMu sync.RWMutex
)

func main() {
	port := "8989"
	addr := ":" + port

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	defer listener.Close()

	log.Printf("TCP server started on %s", addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	go func() {
		<-sigCh
		log.Printf("Shutdown signal received...")

		cancel()
		listener.Close()

		closeAllClients()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.Printf("All connections closed")
		case <-shutdownCtx.Done():
			log.Printf("Shutdown timeout reached. Force closing remaining connections.")
		}
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handleClient(ctx, c)
		}(conn)
	}

	log.Println("Server Stopped")
}

func handleClient(ctx context.Context, conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("Conn close error: %v", err)
		}
	}()

	client := &Client{
		conn: conn,
		name: "Anonymous",
	}

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	writeLine(writer, "Hello! Enter your name here: ")

	name, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	client.name = strings.TrimSpace(name)
	if client.name == "" {
		client.name = "Guest"
	}

	addClient(client)
	defer removeClient(client)

	log.Printf("New connection joined %s (IP: %s)", client.name, client.RemoteAddr())

	broadcast(fmt.Sprintf("*** %s joined the chat ***", client.name), nil)
	writeLine(writer, fmt.Sprintf("Welcome %s! Type /help for commands.", client.name))

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error from %s (%s): %v", client.name, client.RemoteAddr(), err)
			}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch line {
		case "/help":
			showHelp(writer)
		case "/whoami":
			writeLine(writer, fmt.Sprintf("Your IP: %s\r\n", client.RemoteAddr()))
		case "/users":
			showUsers(writer)
		case "/quit":
			writeLine(writer, "Goodbye!\r\n")
			broadcast(fmt.Sprintf("*** %s left the chat ***", client.name), nil)
			return
		default:
			msg := fmt.Sprintf("[%s]: %s", client.name, line)
			log.Printf("[CHAT] %s: %s", client.name, line)

			broadcast(msg, nil)
		}

	}

}

func addClient(c *Client) {
	clientsMu.Lock()
	clients[c] = true
	clientsMu.Unlock()
}

func removeClient(c *Client) {
	clientsMu.Lock()
	delete(clients, c)
	clientsMu.Unlock()
}

func broadcast(message string, exclude *Client) {
	if !strings.HasSuffix(message, "\r\n") {
		message += "\r\n"
	}

	clientsMu.RLock()
	defer clientsMu.RUnlock()

	for c := range clients {
		if exclude != nil && c == exclude {
			continue
		}

		w := bufio.NewWriter(c.conn)
		if _, err := w.WriteString(message); err == nil {
			w.Flush()
		} else {
			log.Printf("Write error to %s: %v", c.name, err)
		}
	}
}

func showHelp(w *bufio.Writer) {
	help := "Available commands:\r\n" +
		"/help    - show this help\r\n" +
		"/whoami  - show your IP address\r\n" +
		"/users   - list all online users\r\n" +
		"/quit    - leave the chat\r\n" +
		"\r\nJust type any message to send it to everyone in the chat."
	writeLine(w, help)
}

func showUsers(w *bufio.Writer) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()

	if len(clients) == 0 {
		writeLine(w, "No users online.\r\n")
		return
	}

	var names []string
	for client := range clients {
		names = append(names, client.name)
	}

	writeLine(w, fmt.Sprintf("Online users (%d): %s\r\n", len(names), strings.Join(names, ", ")))
}

func writeLine(w *bufio.Writer, line string) {
	if !strings.HasSuffix(line, "\r\n") {
		line += "\r\n"
	}
	if _, err := w.WriteString(line); err != nil {
		return
	}
	w.Flush()
}

func (c *Client) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

func closeAllClients() {
	clientsMu.RLock()
	defer clientsMu.RUnlock()

	for c := range clients {
		_ = c.conn.Close()
	}
}
