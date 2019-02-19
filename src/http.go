package src

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
)

/*

  http.ReadRequest()
  http.ReadResponse()
  httputil.DumpRequest()
  httputil.DumpResponse()
*/


func EncodeRequest(r *http.Request) ([]byte, error) {
	raw := bytes.NewBuffer([]byte{})
	// sign
	binary.Write(raw, binary.LittleEndian, []byte("sign"))
	reqBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		return nil, err
	}
	// body length + 1
	binary.Write(raw, binary.LittleEndian, int32(len(reqBytes)+1))
	// http or https
	binary.Write(raw, binary.LittleEndian, bool(r.URL.Scheme == "https"))
	if err := binary.Write(raw, binary.LittleEndian, reqBytes); err != nil {
		return nil, err
	}
	return raw.Bytes(), nil
}

// byte to request
func DecodeRequest(data []byte, port int) (*http.Request, error) {
	if len(data) <= 100 {
		return nil, errors.New("Byte length too small to decode")
	}
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(data[1:])))
	if err != nil {
		return nil, err
	}
	req.Host = "127.0.0.1"
	if port != 80 {
		req.Host += ":" + strconv.Itoa(port)
	}
	scheme := "http"
	if data[0] == 1 {
		scheme = "https"
	}
	req.URL, _ = url.Parse(fmt.Sprintf("%s://%s%s", scheme, req.Host, req.RequestURI))
	req.RequestURI = ""

	return req, nil
}

//// response to byte
func EncodeResponse(r *http.Response) ([]byte, error) {
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, []byte("sign"))
	respBytes, err := httputil.DumpResponse(r, true)
	if err != nil {
		return nil, err
	}
	binary.Write(raw, binary.LittleEndian, int32(len(respBytes)))
	if err := binary.Write(raw, binary.LittleEndian, respBytes); err != nil {
		return nil, err
	}
	return raw.Bytes(), nil
}

//// byte to response
func DecodeResponse(data []byte) (*http.Response, error) {

	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(data)), nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
