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

type Cleint struct {
	conn net.Conn
	name string
}

func (c *Cleint) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

func main() {
	port := "8989"
	addr := ":" + port

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("Failed to listen on %s: %v", addr, err)
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
		log.Printf("Shutdown signal received.Stopping new connections...")

		cancel()
		listener.Close()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

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
			if ctx.Err != nil {
				break
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		log.Printf("New connection from %s", conn.RemoteAddr())

		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handle(ctx, c)
		}(conn)
	}

	log.Println("Server Stopped")
}

func handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	client := &Cleint{
		conn: conn,
		name: conn.RemoteAddr().String(),
	}

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	writeLine(writer, "Hello! Enter your name here: ")

	name, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Failed to read name from %s: %v", client.RemoteAddr())
		return
	}

	client.name = strings.TrimSpace(name)
	if client.name == "" {
		client.name = "default"
	}

	log.Printf("New connection from %s (IP: %s)", client.name, client.RemoteAddr())

	writeLine(writer, fmt.Sprintf("Welcome %s! You can send message: \nType /whoami to see your IP.\r\n", client.name))

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("read error from %s (%s): %v", client.name, client.RemoteAddr(), err)
			}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "/whoami" {
			writeLine(writer, fmt.Sprintf("Your IP: %s\r\n", client.RemoteAddr()))
		}

		log.Printf("[%s] %s", client.name, line)

		response := fmt.Sprintf("[%s] %s\r\n", client.name, line)
		writeLine(writer, response)
	}

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
