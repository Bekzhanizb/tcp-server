package main

import "net"

type Client struct {
	conn net.Conn
	Name string
	out  chan string
}

func (m *Client) ChangeName(name string) {
	m.Name = name
}
