package service

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tigerowo/infinite-canvas/config"
)

func TestExplicitSOCKS5PreservesCreatePollAndDownload(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	old := config.Cfg.OutboundSOCKS5Proxy
	config.Cfg.OutboundSOCKS5Proxy = listener.Addr().String()
	defer func() { config.Cfg.OutboundSOCKS5Proxy = old }()
	seen := make(chan string, 3)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				if err := serveProxyFixture(conn, seen); err != nil {
					t.Error(err)
				}
			}()
		}
	}()
	client := OutboundHTTPClient(2 * time.Second)
	for _, stage := range []struct{ method, path, body string }{
		{"POST", "/v1/videos", `{"model":"fixture-video","prompt":" original "}`},
		{"GET", "/v1/videos/original-task", ""},
		{"GET", "/v1/videos/original-task/content", ""},
	} {
		req, _ := http.NewRequest(stage.method, "http://93.184.216.34"+stage.path, strings.NewReader(stage.body))
		req.Header.Set("Authorization", "Bearer fixture-key")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || string(body) != "fixture-response" {
			t.Fatalf("response %q: %v", body, err)
		}
		select {
		case record := <-seen:
			if record != stage.method+" "+stage.path+" "+stage.body {
				t.Fatal(record)
			}
		case <-time.After(time.Second):
			t.Fatal("proxy not used")
		}
	}
	for _, target := range []string{"http://127.0.0.1/", "http://localhost/", "http://169.254.169.254/"} {
		if response, err := client.Get(target); err == nil {
			response.Body.Close()
			t.Fatal("private target accepted")
		}
	}
	select {
	case record := <-seen:
		t.Fatal("private request reached proxy", record)
	default:
	}
}

func serveProxyFixture(conn net.Conn, seen chan<- string) error {
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil {
		return err
	}
	if greeting[0] != 5 {
		return fmt.Errorf("not SOCKS5")
	}
	if _, err := io.CopyN(io.Discard, conn, int64(greeting[1])); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return err
	}
	var request [10]byte
	if _, err := io.ReadFull(conn, request[:]); err != nil {
		return err
	}
	if request[0] != 5 || request[1] != 1 || request[3] != 1 || !bytes.Equal(request[4:8], net.ParseIP("93.184.216.34").To4()) || binary.BigEndian.Uint16(request[8:]) != 80 {
		return fmt.Errorf("proxy must receive the validated public IP, not a hostname: %v", request)
	}
	if _, err := conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}); err != nil {
		return err
	}
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return err
	}
	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return err
	}
	if req.Header.Get("Authorization") != "Bearer fixture-key" || req.Host != "93.184.216.34" {
		return fmt.Errorf("request headers changed")
	}
	seen <- req.Method + " " + req.URL.Path + " " + string(body)
	_, err = fmt.Fprint(conn, "HTTP/1.1 200 OK\r\nContent-Length: 16\r\nConnection: close\r\n\r\nfixture-response")
	return err
}
