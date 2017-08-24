package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// 1.握手
// 2.获取客户端代理的请求
// 3.开始代理

func readAddr(r *bufio.Reader) (string, error) {
	version, _ := r.ReadByte()
	log.Printf("version:%d", version)
	if version != 5 {
		return "", errors.New("bad version")
	}
	cmd, _ := r.ReadByte()
	log.Printf("cmd:%d", cmd)

	if cmd != 1 {
		return "", errors.New("bad cmd")
	}

	// skip rsv字段
	r.ReadByte()

	addrtype, _ := r.ReadByte()
	log.Printf("addr type:%d", addrtype)
	if addrtype != 3 {
		return "", errors.New("bad addr type")
	}

	// 读取一个字节的数据，代表后面紧跟着的域名的长度
	// 读取n个字节得到域名,n根据上一步得到的结果来决定
	addrlen, _ := r.ReadByte()
	addr := make([]byte, addrlen)
	io.ReadFull(r, addr)
	log.Printf("addr:%s", addr)

	var port int16
	binary.Read(r, binary.BigEndian, &port)

	return fmt.Sprintf("%s:%d", addr, port), nil
}

func handshake(r *bufio.Reader, conn net.Conn) error {
	version, _ := r.ReadByte()
	log.Printf("version:%d", version)
	if version != 5 {
		return errors.New("bad version")
	}
	nmethods, _ := r.ReadByte()
	log.Printf("nmethods:%d", nmethods)

	buf := make([]byte, nmethods)
	io.ReadFull(r, buf)
	log.Printf("%v", buf)

	resp := []byte{5, 0}
	conn.Write(resp)
	return nil
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)

	handshake(r, conn)
	addr, _ := readAddr(r)
	log.Printf("addr:%s", addr)
	resp := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	conn.Write(resp)

	remote, err := net.Dial("tcp", addr)
	if err != nil {
		log.Print(err)
		return
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)
	// go 读取(conn)的数据，发送到remote，直到conn的EOF, 关闭remote
	go func() {
		defer wg.Done()
		io.Copy(remote, r)
		remote.Close()

	}()
	// go 读取remote的数据，发送到客户端(conn)，直到remote的EOF，关闭conn
	go func() {
		defer wg.Done()
		io.Copy(conn, remote)
		conn.Close()
	}()

	// 等待两个协程结束
	wg.Wait()
}

func main() {
	flag.Parse()

	l, err := net.Listen("tcp", ":8022")
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, _ := l.Accept()
		go handleConn(conn)
	}
}
