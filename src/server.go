package src

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type TRPServer struct {
	TcpPort  int
	HttpPort int
	//listener *net.TCPListener
	//conn     *net.TCPConn
	//connMap  map[string]Conn
	sync.Mutex
	Conn
	VerificationKey string
}

type Conn struct{
	*net.TCPConn
	listener *net.TCPListener
	prefix string
}

var (
	connMap = make(map[string]Conn)
	//connMap = new(sync.Map)
)


func NewRPServer() *TRPServer {
	s := new(TRPServer)
	return s
}

func (s *TRPServer) Start() error {
	var err error
	connObj := new(Conn)
	connObj.listener, err = net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP("0.0.0.0"), s.TcpPort, ""})
	if err != nil {
		return err
	}
	go s.startHttpServer()
	return s.startTcpServer(*connObj)
}

func (conn *Conn) CloseConn() error {
	conn.Close()
	conn = nil
	return errors.New("TCP instance not created！")
}

func (s *TRPServer) startTcpServer(connObj Conn) error {
	var err error
	for {
		connObj.TCPConn, err = connObj.listener.AcceptTCP()
		if err != nil {
			log.Println(err)
			continue
		}
		go s.cliProcess(connObj)


	}
	return err
}

func badRequest(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
}

func (s *TRPServer) startHttpServer() {
	fmt.Println("Reverse proxy server is starting...")
	router := httprouter.New()
	//router.GET("/:prefix", s.ProxyHandler)
	router.GET("/:prefix/:filename", s.ProxyHandler)
	log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", s.HttpPort), router))
}


func (s *TRPServer) ProxyHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
	s.Lock()
	defer s.Unlock()
	prefix := ps.ByName("prefix")

	loadPrefixVal,_ := connMap[prefix]
	fmt.Println(connMap)
	fmt.Println("handler prefix: ",loadPrefixVal)
	if conn, ok := connMap[prefix]; !ok {
		log.Println("Invalid prefix!")
		return
	}else{
		log.Println(r.RequestURI)
		err := conn.write(r)
		if err != nil {
			badRequest(w)
			log.Println(err)
			return
		}
		err = conn.read(w,prefix)
		if err != nil {
			badRequest(w)
			log.Println(err)
			return
		}
	}
}


func (s *TRPServer) cliProcess(connObj Conn) error {
	// 5 seconds timeout
	connObj.SetReadDeadline(time.Now().Add(time.Duration(5) * time.Second))
	vval := make([]byte, 20)
	_, err := connObj.Read(vval)
	if err != nil {
		log.Println("Read client timeout! Client address：:", connObj.RemoteAddr())
		connObj.CloseConn()
		return err
	}
	if bytes.Compare(vval, getverifyval(s.VerificationKey)[:]) != 0 {
		log.Println("Failed to verify the current connection code, shut it down!:", connObj.RemoteAddr())
		connObj.CloseConn()
		return err
	}
	// remove timeout
	connObj.SetReadDeadline(time.Time{})

	log.Println("New connection：", connObj.RemoteAddr())
	connObj.SetKeepAlive(true)
	connObj.SetKeepAlivePeriod(time.Duration(10 * time.Second))

	//read prefix from client
	bufferReader := bufio.NewReader(connObj)
	prefixByte ,prefixErr := bufferReader.ReadBytes('\v')
	if prefixErr != nil{
		log.Println("An error occurred while reading prefix")
		connObj.CloseConn()
		return prefixErr
	}
	prefixByte = prefixByte[:len(prefixByte)-1]
	prefix := string(prefixByte)
	if err != nil{
		log.Println("Error reading prefix")
	}

	s.Lock()
	if connObject,ok := connMap[prefix]; ok {
		log.Println("Prefix already in use!")
		delete(connMap,prefix)
		log.Println("Connection object deleted!")
		connObject.CloseConn()
	}
	//connObj.TCPConn = conn
	connMap[prefix] = connObj
	_,ok := connMap[prefix]
	if !ok {
		log.Println("load error")
	}
	log.Println("Prefix inserted!")
	s.Unlock()

	return nil
}


func (conn *Conn) write(r *http.Request) error {

	if conn != nil{
		raw, err := EncodeRequest(r)
		if err != nil {
			return err
		}
		c, err := conn.Write(raw)
		if err != nil {
			return err
		}
		if c != len(raw) {
			return errors.New("Conflicting byte length！")
		}
	}else{
		return errors.New("Client disconnected!")
	}
	return nil
}

func (conn Conn) read(w http.ResponseWriter, prefix string) error {
	val := make([]byte, 4) // flag
	_, err := conn.Read(val)
	fmt.Println(val)
	if err != nil {
		fmt.Println("damn here!!")
		return err
	}
	flags := string(val)
	switch flags {
	case "sign":
		_, err = conn.Read(val)
		if err != nil {
			return err
		}
		nlen := int(binary.LittleEndian.Uint32(val))
		if nlen == 0 {
			return errors.New("Error reading byte length")
		}
		log.Println("Data received，length of bytes to read：", nlen)
		raw := make([]byte, 0)
		buff := make([]byte, 1024)
		c := 0
		for {
			clen, err := conn.Read(buff)
			if err != nil && err != io.EOF {
				return err
			}
			raw = append(raw, buff[:clen]...)
			c += clen
			if c >= nlen {
				break
			}
		}
		log.Println("Read completed，length is：", c, "Actual raw length：", len(raw))
		if c != nlen {
			return fmt.Errorf("Read the wrong length of data，has read %dbyte，need to read %dbyte。", c, nlen)
		}
		resp, err := DecodeResponse(raw)
		if err != nil {
			return err
		}
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		for k, v := range resp.Header {
			for _, v2 := range v {
				w.Header().Set(k, v2)
			}
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(bodyBytes)
	case "msg0":
		return errors.New("An error returned from client！")
	default:
		log.Println("Unknown error：", string(val))
	}

	return nil
}


