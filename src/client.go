package src

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	disabledRedirect = errors.New("disabled redirect.")
	server *http.Server
)

type TRPClient struct {
	ServerAddr  string
	FileServerPort int
	FileServerDir string
	conn     net.Conn
	Prefix string
	VerificationKey string
}



func NewRPClient() *TRPClient {
	c := new(TRPClient)
	return c
}

func (c *TRPClient) Start() error {
	conn, err := net.Dial("tcp", c.ServerAddr)
	if err != nil {
		return err
	}
	conn.(*net.TCPConn).SetKeepAlive(true)
	conn.(*net.TCPConn).SetKeepAlivePeriod(time.Duration(10 * time.Second))
	c.conn = conn
	//go c.StartFileServer()
	//go c.Serve(fmt.Sprintf(":%d",c.FileServerPort))
	return c.process()
}

func (c *TRPClient) werror() {
	c.conn.Write([]byte("msg0"))
}


func (c *TRPClient) process() error {

	if _, err := c.conn.Write(getverifyval(c.VerificationKey)); err != nil {
		return err
	}

	//write prefix to server
	if _, err := c.conn.Write([]byte(c.Prefix+"\v"));err != nil{
		return err
	}

	val := make([]byte, 4)
	for {
		_, err := c.conn.Read(val)
		if err != nil {
			log.Println("read data error：", err)
			return err
		}
		flags := string(val)
		fmt.Println(flags)
		switch flags {
		case "sign":
			_, err := c.conn.Read(val)
			nlen := binary.LittleEndian.Uint32(val)
			log.Println("Data sent from server, length：", nlen)
			if nlen <= 0 {
				log.Println("Invalid data length")
				c.werror()
				continue
			}
			raw := make([]byte, nlen)
			n, err := c.conn.Read(raw)
			if err != nil {
				return err
			}
			if n != int(nlen) {
				log.Printf("Length of received data is incorrect，have read %dbyte，total length%dbytes\n", n, nlen)
				c.werror()
				continue
			}
			req, err := DecodeRequest(raw, c.FileServerPort)
			if err != nil {
				log.Println("DecodeRequest error：", err)
				c.werror()
				continue
			}

			rawQuery := ""
			if req.URL.RawQuery != "" {
				rawQuery = "?" + req.URL.RawQuery
			}
			log.Println(req.URL.Path + rawQuery)

			client := new(http.Client)
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return disabledRedirect
			}
			resp, err := client.Do(req)

			disRedirect := err != nil && strings.Contains(err.Error(), disabledRedirect.Error())
			if err != nil && !disRedirect {
				log.Println("Request local file server improperly：", err)
				c.werror()
				continue
			}
			if !disRedirect {
				defer resp.Body.Close()
			} else {
				resp.Body = nil
				resp.ContentLength = 0
			}
			respBytes, err := EncodeResponse(resp)
			if err != nil {
				log.Println("EncodeResponse error：", err)
				c.werror()
				continue
			}
			n, err = c.conn.Write(respBytes)

			if err != nil {
				log.Println("Failed to send data：", err)
			}
			if n != len(respBytes) {
				log.Printf("Incorrect sending data length，has sent：%dbyte，total：%dbyte\n", n, len(respBytes))
			} else {
				log.Printf("Request completed，total sent：%dbyte\n", n)
			}

		case "msg0":
			log.Println("An error returned from server")
		default:
		}
	}
	return nil
}

func (c *TRPClient) StartFileServer() {

	http.Handle("/"+c.Prefix+"/", http.StripPrefix("/"+c.Prefix+"/", http.FileServer(http.Dir(c.FileServerDir))))
	err := http.ListenAndServe(fmt.Sprintf(":%d",c.FileServerPort), nil)

	log.Println(err)
}

func (c *TRPClient) Close() error {

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}

	return errors.New("Failed to create TCP instance!")
}


