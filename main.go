package main

import (
	"bufio"
	"fmt"
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
	defer conn.Close()
	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	writer.WriteString(fmt.Sprintf("Hello\r\n"))
	writer.Flush()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		fmt.Println(line)
		writer.WriteString(line + "\r\n")
		writer.Flush()
	}

}
