package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
)

func main() {
	port := "8989"

	listen, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			panic(err)
		}
		fmt.Println(conn.RemoteAddr().String())
		go handle(conn)
	}

}

func handle(conn net.Conn) {
	defer func() {
		if cerr := conn.Close(); cerr != nil && cerr != net.ErrClosed {
			log.Printf("close error: %v", cerr)
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
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("read error from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}
		log.Print(line)

		if _, err := writer.WriteString(line); err != nil {
			return
		}
		if err := writer.Flush(); err != nil {
			return
		}
	}

}
