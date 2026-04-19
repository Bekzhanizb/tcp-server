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
	"sync"
	"syscall"
	"time"
)

func main() {
	port := "8989"
	addr := ":" + port

	listener, err := net.Listen("tcp", ":"+port)
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

}

func handle(ctx context.Context, conn net.Conn) {
	defer func() {
		if cerr := conn.Close(); cerr != nil && cerr != net.ErrClosed {
			log.Printf("close error from %s: %v", conn.RemoteAddr(), cerr)
		}
	}()

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	if _, err := writer.WriteString(fmt.Sprintf("Hello\r\n")); err != nil {
		return
	}
	if err := writer.Flush(); err != nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("read error from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}

		log.Printf("Received from %s: %v", conn.RemoteAddr(), line[:len(line)-1])

		if _, err := writer.WriteString(line); err != nil {
			return
		}
		if err := writer.Flush(); err != nil {
			return
		}
	}

}
